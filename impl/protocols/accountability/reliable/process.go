package reliable

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"math"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/utils"
	"time"
)

type WitnessStage int32

const (
	InitialWitnessStage WitnessStage = iota
	SentEchoFromWitness
	SentReadyFromWitness
	SentValidate
)

type Stage int32

const (
	InitialStage Stage = iota
	SentEchoFromProcess
	SentReadyFromProcess
)

type RecoveryStage int32

const (
	InitialRecoveryStage RecoveryStage = iota
	SentRecover
	SentEcho
	SentReady
)

type messageState struct {
	echoFromProcesses     map[string]bool
	readyFromProcesses    map[string]bool
	readyFromWitnesses    map[string]bool
	validateFromWitnesses map[string]bool

	echoFromProcessesStat  map[int64]int
	readyFromProcessesStat map[int64]int
	readyFromWitnessesStat map[int64]int
	validatesStat          map[int64]int

	stage        Stage
	witnessStage WitnessStage

	ownWitnessSet map[string]bool
	potWitnessSet map[string]bool

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)

	ms.echoFromProcesses = make(map[string]bool)
	ms.readyFromProcesses = make(map[string]bool)
	ms.readyFromWitnesses = make(map[string]bool)
	ms.validateFromWitnesses = make(map[string]bool)

	ms.echoFromProcessesStat = make(map[int64]int)
	ms.readyFromWitnessesStat = make(map[int64]int)
	ms.readyFromProcessesStat = make(map[int64]int)
	ms.validatesStat = make(map[int64]int)

	ms.stage = InitialStage
	ms.witnessStage = InitialWitnessStage

	ms.receivedMessagesCnt = 0

	return ms
}

type recoveryMessageState struct {
	receivedRecover map[string]bool
	receivedReply   map[string]bool
	receivedEcho    map[string]bool
	receivedReady   map[string]bool

	recoverValues map[int64]bool

	replyMessagesStat map[int64]int
	recoverReadyStat  map[int64]int
	echoMessagesStat  map[int64]int
	readyMessagesStat map[int64]int

	stage RecoveryStage

	receivedMessagesCnt int
}

func newRecoveryMessageState() *recoveryMessageState {
	ms := new(recoveryMessageState)

	ms.receivedRecover = make(map[string]bool)
	ms.receivedReply = make(map[string]bool)
	ms.receivedEcho = make(map[string]bool)
	ms.receivedReady = make(map[string]bool)

	ms.recoverValues = make(map[int64]bool)
	ms.replyMessagesStat = make(map[int64]int)
	ms.recoverReadyStat = make(map[int64]int)
	ms.echoMessagesStat = make(map[int64]int)
	ms.readyMessagesStat = make(map[int64]int)

	ms.stage = InitialRecoveryStage

	ms.receivedMessagesCnt = 0

	return ms
}

type Process struct {
	actorPid  *actor.PID
	pid       string
	actorPids map[string]*actor.PID

	transactionCounter int64
	messageCounter     int64

	deliveredMessages        map[string]map[int64]int64
	deliveredMessagesHistory []string
	messagesLog              map[string]map[int64]*messageState
	lastSentPMessages        map[string]map[int64]*messages.ReliableProtocolMessage
	recoveryMessagesLog      map[string]map[int64]*recoveryMessageState

	quorumThreshold         int
	readyMessagesThreshold  int
	recoverySwitchTimeoutNs time.Duration
	witnessThreshold        int

	wSelector   *hashing.WitnessesSelector
	historyHash *hashing.HistoryHash

	logger             *eventlogger.EventLogger
	transactionManager *protocols.TransactionManager
}

func (p *Process) InitProcess(
	actorPid *actor.PID,
	actorPids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger,
	transactionManager *protocols.TransactionManager,
) {
	p.actorPid = actorPid
	p.pid = utils.MakeCustomPid(actorPid)
	p.actorPids = make(map[string]*actor.PID)
	p.transactionCounter = 0
	p.messageCounter = 0

	p.quorumThreshold = int(math.Ceil(float64(len(actorPids)+parameters.FaultyProcesses+1) / float64(2)))
	p.readyMessagesThreshold = parameters.FaultyProcesses + 1
	p.recoverySwitchTimeoutNs = time.Duration(parameters.RecoverySwitchTimeoutNs)
	p.witnessThreshold = parameters.WitnessThreshold

	p.deliveredMessages = make(map[string]map[int64]int64)
	p.messagesLog = make(map[string]map[int64]*messageState)
	p.lastSentPMessages = make(map[string]map[int64]*messages.ReliableProtocolMessage)
	p.recoveryMessagesLog = make(map[string]map[int64]*recoveryMessageState)

	pids := make([]string, len(actorPids))
	for i, currActorPid := range actorPids {
		pid := utils.MakeCustomPid(currActorPid)
		pids[i] = pid
		p.actorPids[pid] = currActorPid
		p.deliveredMessages[pid] = make(map[int64]int64)
		p.messagesLog[pid] = make(map[int64]*messageState)
		p.lastSentPMessages[pid] = make(map[int64]*messages.ReliableProtocolMessage)
		p.recoveryMessagesLog[pid] = make(map[int64]*recoveryMessageState)
	}

	var hasher hashing.Hasher
	if parameters.NodeIdSize == 256 {
		hasher = hashing.HashSHA256{}
	} else {
		hasher = hashing.HashSHA512{}
	}

	p.wSelector = &hashing.WitnessesSelector{
		NodeIds:              pids,
		Hasher:               hasher,
		MinPotWitnessSetSize: parameters.MinPotWitnessSetSize,
		MinOwnWitnessSetSize: parameters.MinOwnWitnessSetSize,
		PotWitnessSetRadius:  parameters.PotWitnessSetRadius,
		OwnWitnessSetRadius:  parameters.OwnWitnessSetRadius,
	}
	binCapacity := uint(math.Pow(2, float64(parameters.NodeIdSize/parameters.NumberOfBins)))
	p.historyHash = hashing.NewHistoryHash(uint(parameters.NumberOfBins), binCapacity, hasher)

	p.logger = eventlogger.InitEventLogger(p.pid, logger)
	p.transactionManager = transactionManager
}

func (p *Process) initMessageState(
	context actor.SenderContext,
	broadcastInstance *messages.BroadcastInstance,
	value int64,
) *messageState {
	msgState := newMessageState()
	p.messagesLog[broadcastInstance.Author][broadcastInstance.SeqNumber] = msgState

	p.logger.OnHistoryUsedInWitnessSetSelection(
		broadcastInstance, p.historyHash, p.deliveredMessagesHistory)
	msgState.ownWitnessSet, msgState.potWitnessSet =
		p.wSelector.GetWitnessSet(broadcastInstance.Author, broadcastInstance.SeqNumber, p.historyHash)

	p.logger.OnWitnessSetSelected("own", broadcastInstance, msgState.ownWitnessSet)
	p.logger.OnWitnessSetSelected("pot", broadcastInstance, msgState.potWitnessSet)

	p.broadcastToWitnesses(
		context,
		broadcastInstance,
		&messages.ReliableProtocolMessage{
			Stage: messages.ReliableProtocolMessage_NOTIFY,
			Value: value,
		},
		msgState)

	return msgState
}

func (p *Process) registerMessage(
	context actor.Context,
	bInstance *messages.BroadcastInstance,
	value int64,
) *messageState {
	msgState := p.messagesLog[bInstance.Author][bInstance.SeqNumber]
	if msgState == nil {
		msgState = p.initMessageState(context, bInstance, value)
		context.ReenterAfter(
			actor.NewFuture(context.ActorSystem(), p.recoverySwitchTimeoutNs),
			func(res interface{}, err error) {
				if !p.delivered(bInstance, value) {
					p.logger.OnRecoveryProtocolSwitch(bInstance)
					recoveryState := p.initRecoveryMessageState(bInstance)
					p.broadcastRecover(context, bInstance, recoveryState)
				}
			})
	}
	return msgState
}

func (p *Process) initRecoveryMessageState(
	bInstance *messages.BroadcastInstance,
) *recoveryMessageState {
	recoveryState := p.recoveryMessagesLog[bInstance.Author][bInstance.SeqNumber]

	if recoveryState == nil {
		recoveryState = newRecoveryMessageState()
		p.recoveryMessagesLog[bInstance.Author][bInstance.SeqNumber] = recoveryState
	}

	return recoveryState
}

func (p *Process) sendProtocolMessage(
	context actor.SenderContext,
	to *actor.PID,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_ReliableProtocolMessage{
			ReliableProtocolMessage: reliableMessage.Copy(),
		},
		Stamp: p.messageCounter,
	}

	context.RequestWithCustomSender(to, bMessage, p.actorPid)
	p.logger.OnMessageSent(p.messageCounter)

	p.messageCounter++
}

func (p *Process) sendRecoveryMessage(
	context actor.SenderContext,
	to *actor.PID,
	bInstance *messages.BroadcastInstance,
	recoveryMessage *messages.RecoveryProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_RecoveryProtocolMessage{
			RecoveryProtocolMessage: recoveryMessage.Copy(),
		},
		Stamp: p.messageCounter,
	}

	context.RequestWithCustomSender(to, bMessage, p.actorPid)
	p.logger.OnMessageSent(p.messageCounter)

	p.messageCounter++
}

func (p *Process) broadcastProtocolMessage(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	message *messages.ReliableProtocolMessage,
) {
	for _, pid := range p.actorPids {
		p.sendProtocolMessage(context, pid, bInstance, message)
	}
}

func (p *Process) broadcastRecoveryMessage(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	message *messages.RecoveryProtocolMessage,
) {
	for _, pid := range p.actorPids {
		p.sendRecoveryMessage(context, pid, bInstance, message)
	}
}

func (p *Process) broadcastToWitnesses(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	message *messages.ReliableProtocolMessage,
	msgState *messageState,
) {
	for pid := range msgState.potWitnessSet {
		p.sendProtocolMessage(context, p.actorPids[pid], bInstance, message)
	}

	p.lastSentPMessages[bInstance.Author][bInstance.SeqNumber] = message
}

func (p *Process) broadcastReadyFromWitness(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	value int64,
	msgState *messageState,
) {
	p.broadcastProtocolMessage(
		context,
		bInstance,
		&messages.ReliableProtocolMessage{
			Stage: messages.ReliableProtocolMessage_READY_FROM_WITNESS,
			Value: value,
		})
	msgState.witnessStage = SentReadyFromWitness
}

func (p *Process) broadcastRecover(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	recoveryState *recoveryMessageState,
) {
	lastProcessMessage := p.lastSentPMessages[bInstance.Author][bInstance.SeqNumber]

	p.broadcastRecoveryMessage(
		context,
		bInstance,
		&messages.RecoveryProtocolMessage{
			Stage:                   messages.RecoveryProtocolMessage_RECOVER,
			ReliableProtocolMessage: lastProcessMessage,
		})

	recoveryState.stage = SentRecover
}

func (p *Process) broadcastEcho(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
	recoveryState *recoveryMessageState,
) {
	p.broadcastRecoveryMessage(
		context,
		bInstance,
		&messages.RecoveryProtocolMessage{
			Stage:                   messages.RecoveryProtocolMessage_ECHO,
			ReliableProtocolMessage: reliableMessage,
		})

	recoveryState.stage = SentEcho
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
	recoveryState *recoveryMessageState,
) {
	p.broadcastRecoveryMessage(
		context,
		bInstance,
		&messages.RecoveryProtocolMessage{
			Stage:                   messages.RecoveryProtocolMessage_READY,
			ReliableProtocolMessage: reliableMessage,
		})

	recoveryState.stage = SentReady
}

func (p *Process) isWitness(msgState *messageState) bool {
	return msgState.potWitnessSet[p.pid]
}

func (p *Process) delivered(
	bInstance *messages.BroadcastInstance,
	value int64,
) bool {
	deliveredValue, delivered :=
		p.deliveredMessages[bInstance.Author][bInstance.SeqNumber]

	if delivered && deliveredValue != value {
		p.logger.OnAttack(bInstance, value, deliveredValue)
	}

	return delivered
}

func (p *Process) deliver(
	bInstance *messages.BroadcastInstance,
	value int64,
) {
	p.deliveredMessages[bInstance.Author][bInstance.SeqNumber] = value
	p.deliveredMessagesHistory = append(p.deliveredMessagesHistory, bInstance.ToString())
	p.historyHash.Insert(
		utils.TransactionToBytes(bInstance.Author, bInstance.SeqNumber))

	messagesReceived := 0

	msgState := p.messagesLog[bInstance.Author][bInstance.SeqNumber]
	if msgState != nil {
		messagesReceived += msgState.receivedMessagesCnt
		delete(p.messagesLog[bInstance.Author], bInstance.SeqNumber)
	}

	recoveryMsgState := p.recoveryMessagesLog[bInstance.Author][bInstance.SeqNumber]
	// TODO(me) - does it make sense?
	if recoveryMsgState != nil {
		messagesReceived += recoveryMsgState.receivedMessagesCnt
	}

	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *Process) processReliableProtocolMessage(
	context actor.Context,
	senderPid string,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
) {
	value := reliableMessage.Value

	if p.delivered(bInstance, value) {
		return
	}

	msgState := p.registerMessage(context, bInstance, value)
	msgState.receivedMessagesCnt++

	switch reliableMessage.Stage {
	case messages.ReliableProtocolMessage_NOTIFY:
		if !p.isWitness(msgState) || msgState.witnessStage >= SentEchoFromWitness {
			return
		}
		p.broadcastProtocolMessage(
			context,
			bInstance,
			&messages.ReliableProtocolMessage{
				Stage: messages.ReliableProtocolMessage_ECHO_FROM_WITNESS,
				Value: value,
			})
		msgState.witnessStage = SentEchoFromWitness
	case messages.ReliableProtocolMessage_ECHO_FROM_WITNESS:
		if !msgState.ownWitnessSet[senderPid] || msgState.stage >= SentEchoFromProcess {
			return
		}
		p.broadcastToWitnesses(
			context,
			bInstance,
			&messages.ReliableProtocolMessage{
				Stage: messages.ReliableProtocolMessage_ECHO_FROM_PROCESS,
				Value: value,
			},
			msgState)
		msgState.stage = SentEchoFromProcess
	case messages.ReliableProtocolMessage_ECHO_FROM_PROCESS:
		if !p.isWitness(msgState) ||
			msgState.witnessStage >= SentReadyFromWitness ||
			msgState.echoFromProcesses[senderPid] {
			return
		}

		msgState.echoFromProcesses[senderPid] = true
		msgState.echoFromProcessesStat[value]++

		if msgState.echoFromProcessesStat[value] >= p.quorumThreshold {
			p.broadcastReadyFromWitness(context, bInstance, value, msgState)
		}
	case messages.ReliableProtocolMessage_READY_FROM_WITNESS:
		if !msgState.ownWitnessSet[senderPid] ||
			msgState.stage >= SentReadyFromProcess ||
			msgState.readyFromWitnesses[senderPid] {
			return
		}

		msgState.readyFromWitnesses[senderPid] = true
		msgState.readyFromWitnessesStat[value]++

		if msgState.readyFromWitnessesStat[value] >= p.witnessThreshold {
			p.broadcastToWitnesses(
				context,
				bInstance,
				&messages.ReliableProtocolMessage{
					Stage: messages.ReliableProtocolMessage_READY_FROM_PROCESS,
					Value: value,
				},
				msgState)
			msgState.stage = SentReadyFromProcess
		}
	case messages.ReliableProtocolMessage_READY_FROM_PROCESS:
		if !p.isWitness(msgState) || msgState.readyFromProcesses[senderPid] {
			return
		}

		msgState.readyFromProcesses[senderPid] = true
		msgState.readyFromProcessesStat[value]++

		if msgState.witnessStage < SentReadyFromWitness &&
			msgState.readyFromProcessesStat[value] >= p.readyMessagesThreshold {
			p.broadcastReadyFromWitness(context, bInstance, value, msgState)
		}

		if msgState.witnessStage < SentValidate &&
			msgState.readyFromProcessesStat[value] >= p.quorumThreshold {
			p.broadcastProtocolMessage(
				context,
				bInstance,
				&messages.ReliableProtocolMessage{
					Stage: messages.ReliableProtocolMessage_VALIDATE,
					Value: value,
				})
			msgState.witnessStage = SentValidate
		}
	case messages.ReliableProtocolMessage_VALIDATE:
		if !msgState.ownWitnessSet[senderPid] || msgState.validateFromWitnesses[senderPid] {
			return
		}

		msgState.validateFromWitnesses[senderPid] = true
		msgState.validatesStat[value]++

		if msgState.validatesStat[value] >= p.witnessThreshold {
			recoveryState := p.recoveryMessagesLog[bInstance.Author][bInstance.SeqNumber]

			if recoveryState != nil {
				for pid := range recoveryState.receivedRecover {
					p.sendRecoveryMessage(
						context,
						p.actorPids[pid],
						bInstance,
						&messages.RecoveryProtocolMessage{
							Stage:                   messages.RecoveryProtocolMessage_REPLY,
							ReliableProtocolMessage: reliableMessage,
						})
				}
			}

			p.deliver(bInstance, value)
		}
	}
}

func makeReliableProtocolMessage(value int64) *messages.ReliableProtocolMessage {
	return &messages.ReliableProtocolMessage{
		Stage: messages.ReliableProtocolMessage_NOTIFY,
		Value: value,
	}
}

func (p *Process) processRecoveryProtocolMessage(
	context actor.Context,
	senderPid string,
	bInstance *messages.BroadcastInstance,
	recoveryMessage *messages.RecoveryProtocolMessage,
) {
	reliableMessage := recoveryMessage.ReliableProtocolMessage

	deliveredValue, delivered :=
		p.deliveredMessages[bInstance.Author][bInstance.SeqNumber]

	recoveryState := p.initRecoveryMessageState(bInstance)
	recoveryState.receivedMessagesCnt++

	switch recoveryMessage.Stage {
	case messages.RecoveryProtocolMessage_RECOVER:
		if recoveryState.receivedRecover[senderPid] {
			return
		}

		recoveryState.receivedRecover[senderPid] = true
		recoverMessagesCnt := len(recoveryState.receivedRecover)

		if delivered {
			p.sendRecoveryMessage(
				context,
				p.actorPids[senderPid],
				bInstance,
				&messages.RecoveryProtocolMessage{
					Stage:                   messages.RecoveryProtocolMessage_REPLY,
					ReliableProtocolMessage: makeReliableProtocolMessage(deliveredValue),
				},
			)
		}

		if recoveryState.stage < SentRecover &&
			recoverMessagesCnt >= p.readyMessagesThreshold {
			p.broadcastRecover(context, bInstance, recoveryState)
		}

		if reliableMessage != nil {
			recoveryState.recoverValues[reliableMessage.Value] = true
			if reliableMessage.Stage == messages.ReliableProtocolMessage_READY_FROM_PROCESS {
				recoveryState.recoverReadyStat[reliableMessage.Value]++

				if recoveryState.stage < SentEcho &&
					recoveryState.recoverReadyStat[reliableMessage.Value] >= p.readyMessagesThreshold {
					p.broadcastEcho(context, bInstance, reliableMessage, recoveryState)
				}
			}
		}

		if recoveryState.stage < SentEcho &&
			recoverMessagesCnt >= p.quorumThreshold &&
			len(recoveryState.recoverValues) == 1 {
			for value := range recoveryState.recoverValues {
				p.broadcastEcho(
					context,
					bInstance,
					makeReliableProtocolMessage(value),
					recoveryState)
			}
		}
	case messages.RecoveryProtocolMessage_REPLY:
		if recoveryState.receivedReply[senderPid] || reliableMessage == nil {
			return
		}

		recoveryState.receivedReply[senderPid] = true
		recoveryState.replyMessagesStat[reliableMessage.Value]++

		if !delivered &&
			recoveryState.replyMessagesStat[reliableMessage.Value] >= p.readyMessagesThreshold {
			p.deliver(bInstance, reliableMessage.Value)
		}
	case messages.RecoveryProtocolMessage_ECHO:
		if recoveryState.receivedEcho[senderPid] || reliableMessage == nil {
			return
		}

		recoveryState.receivedEcho[senderPid] = true
		recoveryState.echoMessagesStat[reliableMessage.Value]++

		if recoveryState.stage < SentReady &&
			recoveryState.echoMessagesStat[reliableMessage.Value] >= p.quorumThreshold {
			p.broadcastReady(context, bInstance, reliableMessage, recoveryState)
		}
	case messages.RecoveryProtocolMessage_READY:
		if recoveryState.receivedReady[senderPid] || reliableMessage == nil {
			return
		}

		recoveryState.receivedReady[senderPid] = true
		recoveryState.readyMessagesStat[reliableMessage.Value]++

		if recoveryState.stage < SentReady &&
			recoveryState.readyMessagesStat[reliableMessage.Value] >= p.readyMessagesThreshold {
			p.broadcastReady(context, bInstance, reliableMessage, recoveryState)
		}

		if !delivered &&
			recoveryState.readyMessagesStat[reliableMessage.Value] >= p.quorumThreshold {
			p.deliver(bInstance, reliableMessage.Value)
		}
	}
}

func (p *Process) Receive(context actor.Context) {
	message := context.Message()
	switch message.(type) {
	case *messages.Simulate:
		p.transactionManager.Simulate(context, p)
	case *messages.BroadcastInstanceMessage:
		msg := message.(*messages.BroadcastInstanceMessage)
		bInstance := msg.BroadcastInstance

		senderPid := utils.MakeCustomPid(context.Sender())
		p.logger.OnMessageReceived(senderPid, msg.Stamp)

		switch protocolMessage := msg.Message.(type) {
		case *messages.BroadcastInstanceMessage_ReliableProtocolMessage:
			p.processReliableProtocolMessage(
				context,
				senderPid,
				bInstance,
				protocolMessage.ReliableProtocolMessage,
			)
		case *messages.BroadcastInstanceMessage_RecoveryProtocolMessage:
			p.processRecoveryProtocolMessage(
				context,
				senderPid,
				bInstance,
				protocolMessage.RecoveryProtocolMessage,
			)
		}
	}
}

func (p *Process) Broadcast(context actor.SenderContext, value int64) {
	broadcastInstance := &messages.BroadcastInstance{
		Author:    p.pid,
		SeqNumber: p.transactionCounter,
	}

	p.sendProtocolMessage(
		context,
		p.actorPid,
		broadcastInstance,
		&messages.ReliableProtocolMessage{
			Stage: messages.ReliableProtocolMessage_NOTIFY,
			Value: value,
		})

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
