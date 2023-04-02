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

	echoFromProcessesStat  map[protocols.ValueType]int
	readyFromProcessesStat map[protocols.ValueType]int
	readyFromWitnessesStat map[protocols.ValueType]int
	validatesStat          map[protocols.ValueType]int

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

	ms.echoFromProcessesStat = make(map[protocols.ValueType]int)
	ms.readyFromWitnessesStat = make(map[protocols.ValueType]int)
	ms.readyFromProcessesStat = make(map[protocols.ValueType]int)
	ms.validatesStat = make(map[protocols.ValueType]int)

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

	recoverMessagesStat map[protocols.ValueType]int
	replyMessagesStat   map[protocols.ValueType]int
	recoverReadyStat    map[protocols.ValueType]int
	echoMessagesStat    map[protocols.ValueType]int
	readyMessagesStat   map[protocols.ValueType]int

	stage RecoveryStage

	receivedMessagesCnt int
}

func newRecoveryMessageState() *recoveryMessageState {
	ms := new(recoveryMessageState)

	ms.receivedRecover = make(map[string]bool)
	ms.receivedReply = make(map[string]bool)

	ms.recoverMessagesStat = make(map[protocols.ValueType]int)
	ms.replyMessagesStat = make(map[protocols.ValueType]int)

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

	deliveredMessages   map[string]map[int64]protocols.ValueType
	messagesLog         map[string]map[int64]*messageState
	lastSentPMessages   map[string]map[int64]*messages.ReliableProtocolMessage
	recoveryMessagesLog map[string]map[int64]*recoveryMessageState

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

	p.deliveredMessages = make(map[string]map[int64]protocols.ValueType)
	p.messagesLog = make(map[string]map[int64]*messageState)
	p.lastSentPMessages = make(map[string]map[int64]*messages.ReliableProtocolMessage)
	p.recoveryMessagesLog = make(map[string]map[int64]*recoveryMessageState)

	pids := make([]string, len(actorPids))
	for i, currActorPid := range actorPids {
		pid := utils.MakeCustomPid(currActorPid)
		pids[i] = pid
		p.actorPids[pid] = currActorPid
		p.deliveredMessages[pid] = make(map[int64]protocols.ValueType)
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
	sourceMessage *messages.SourceMessage,
) *messageState {
	msgState := newMessageState()
	msgState.ownWitnessSet, msgState.potWitnessSet =
		p.wSelector.GetWitnessSet(sourceMessage.Author, sourceMessage.SeqNumber, p.historyHash)
	p.messagesLog[sourceMessage.Author][sourceMessage.SeqNumber] = msgState

	p.logger.OnWitnessSetSelected("own", sourceMessage, msgState.ownWitnessSet)
	p.logger.OnWitnessSetSelected("pot", sourceMessage, msgState.potWitnessSet)

	p.broadcastToWitnesses(
		context,
		&messages.ReliableProtocolMessage{
			Stage:         messages.ReliableProtocolMessage_NOTIFY,
			SourceMessage: sourceMessage,
		},
		msgState)

	return msgState
}

func (p *Process) registerMessage(
	context actor.Context,
	sourceMessage *messages.SourceMessage,
) *messageState {
	msgState := p.messagesLog[sourceMessage.Author][sourceMessage.SeqNumber]
	if msgState == nil {
		msgState = p.initMessageState(context, sourceMessage)
		context.ReenterAfter(
			actor.NewFuture(context.ActorSystem(), p.recoverySwitchTimeoutNs),
			func(res interface{}, err error) {
				if !p.delivered(sourceMessage) {
					p.logger.OnRecoveryProtocolSwitch(sourceMessage)
					recoveryState := p.initRecoveryMessageState(sourceMessage)
					p.broadcastRecover(context, sourceMessage, recoveryState)
				}
			})
	}
	return msgState
}

func (p *Process) initRecoveryMessageState(
	sourceMessage *messages.SourceMessage,
) *recoveryMessageState {
	recoveryState := p.recoveryMessagesLog[sourceMessage.Author][sourceMessage.SeqNumber]

	if recoveryState == nil {
		recoveryState = newRecoveryMessageState()
		p.recoveryMessagesLog[sourceMessage.Author][sourceMessage.SeqNumber] = recoveryState
	}

	return recoveryState
}

func (p *Process) sendProtocolMessage(
	context actor.SenderContext,
	to *actor.PID,
	message *messages.ReliableProtocolMessage,
) {
	message.Stamp = p.messageCounter

	context.RequestWithCustomSender(to, message.Copy(), p.actorPid)
	p.logger.OnMessageSent(p.messageCounter)

	p.messageCounter++
}

func (p *Process) sendRecoveryMessage(
	context actor.SenderContext,
	to *actor.PID,
	message *messages.RecoveryProtocolMessage,
) {
	message.Stamp = p.messageCounter

	context.RequestWithCustomSender(to, message.Copy(), p.actorPid)
	p.logger.OnMessageSent(p.messageCounter)

	p.messageCounter++
}

func (p *Process) broadcastProtocolMessage(
	context actor.SenderContext,
	message *messages.ReliableProtocolMessage,
) {
	for _, pid := range p.actorPids {
		p.sendProtocolMessage(context, pid, message)
	}
}

func (p *Process) broadcastRecoveryMessage(
	context actor.SenderContext,
	message *messages.RecoveryProtocolMessage,
) {
	for _, pid := range p.actorPids {
		p.sendRecoveryMessage(context, pid, message)
	}
}

func (p *Process) broadcastToWitnesses(
	context actor.SenderContext,
	message *messages.ReliableProtocolMessage,
	msgState *messageState,
) {
	for pid := range msgState.potWitnessSet {
		p.sendProtocolMessage(context, p.actorPids[pid], message)
	}

	sourceMessage := message.SourceMessage
	p.lastSentPMessages[sourceMessage.Author][sourceMessage.SeqNumber] = message
}

func (p *Process) broadcastReadyFromWitness(
	context actor.SenderContext,
	sourceMessage *messages.SourceMessage,
	msgState *messageState,
) {
	p.broadcastProtocolMessage(
		context,
		&messages.ReliableProtocolMessage{
			Stage:         messages.ReliableProtocolMessage_READY_FROM_WITNESS,
			SourceMessage: sourceMessage,
		})
	msgState.witnessStage = SentReadyFromWitness
}

func (p *Process) broadcastRecover(
	context actor.SenderContext,
	sourceMessage *messages.SourceMessage,
	recoveryState *recoveryMessageState,
) {
	lastProcessMessage :=
		p.lastSentPMessages[sourceMessage.Author][sourceMessage.SeqNumber]

	p.broadcastRecoveryMessage(
		context,
		&messages.RecoveryProtocolMessage{
			RecoveryStage: messages.RecoveryProtocolMessage_RECOVER,
			Message:       lastProcessMessage,
		})

	recoveryState.stage = SentRecover
}

func (p *Process) broadcastEcho(
	context actor.SenderContext,
	reliableMessage *messages.ReliableProtocolMessage,
	recoveryState *recoveryMessageState,
) {
	p.broadcastRecoveryMessage(
		context,
		&messages.RecoveryProtocolMessage{
			RecoveryStage: messages.RecoveryProtocolMessage_ECHO,
			Message:       reliableMessage,
		})

	recoveryState.stage = SentEcho
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	reliableMessage *messages.ReliableProtocolMessage,
	recoveryState *recoveryMessageState,
) {
	p.broadcastRecoveryMessage(
		context,
		&messages.RecoveryProtocolMessage{
			RecoveryStage: messages.RecoveryProtocolMessage_READY,
			Message:       reliableMessage,
		})

	recoveryState.stage = SentReady
}

func (p *Process) isWitness(msgState *messageState) bool {
	return msgState.potWitnessSet[p.pid]
}

func taggedWithP(msg *messages.ReliableProtocolMessage) bool {
	return msg.Stage == messages.ReliableProtocolMessage_NOTIFY ||
		msg.Stage == messages.ReliableProtocolMessage_ECHO_FROM_PROCESS ||
		msg.Stage == messages.ReliableProtocolMessage_READY_FROM_PROCESS
}

func (p *Process) delivered(sourceMessage *messages.SourceMessage) bool {
	deliveredValue, delivered :=
		p.deliveredMessages[sourceMessage.Author][sourceMessage.SeqNumber]

	value := protocols.ValueType(sourceMessage.Value)
	if delivered && deliveredValue != value {
		p.logger.OnAttack(sourceMessage, int64(deliveredValue))
	}

	return delivered
}

func (p *Process) deliver(sourceMessage *messages.SourceMessage) {
	p.deliveredMessages[sourceMessage.Author][sourceMessage.SeqNumber] =
		protocols.ValueType(sourceMessage.Value)
	p.historyHash.Insert(
		utils.TransactionToBytes(sourceMessage.Author, sourceMessage.SeqNumber))

	messagesReceived := 0
	msgState := p.messagesLog[sourceMessage.Author][sourceMessage.SeqNumber]
	recoveryMsgState := p.recoveryMessagesLog[sourceMessage.Author][sourceMessage.SeqNumber]

	if msgState != nil {
		messagesReceived += msgState.receivedMessagesCnt
		delete(p.messagesLog[sourceMessage.Author], sourceMessage.SeqNumber)
	}
	if recoveryMsgState != nil {
		messagesReceived += recoveryMsgState.receivedMessagesCnt
		delete(p.recoveryMessagesLog[sourceMessage.Author], sourceMessage.SeqNumber)
	}

	p.logger.OnDeliver(sourceMessage, messagesReceived)
	p.logger.OnHistoryHashUpdate(sourceMessage, p.historyHash)
}

func (p *Process) Receive(context actor.Context) {
	message := context.Message()
	switch message.(type) {
	case *messages.Simulate:
		p.transactionManager.Simulate(context, p)
	case *messages.ReliableProtocolMessage:
		msg := message.(*messages.ReliableProtocolMessage)
		sourceMessage := msg.SourceMessage
		senderPid := utils.MakeCustomPid(context.Sender())
		value := protocols.ValueType(sourceMessage.Value)

		p.logger.OnMessageReceived(senderPid, msg.Stamp)

		if p.delivered(sourceMessage) {
			return
		}

		msgState := p.registerMessage(context, sourceMessage)
		msgState.receivedMessagesCnt++

		switch msg.Stage {
		case messages.ReliableProtocolMessage_NOTIFY:
			if !p.isWitness(msgState) || msgState.witnessStage >= SentEchoFromWitness {
				return
			}
			p.broadcastProtocolMessage(
				context,
				&messages.ReliableProtocolMessage{
					Stage:         messages.ReliableProtocolMessage_ECHO_FROM_WITNESS,
					SourceMessage: sourceMessage,
				})
			msgState.witnessStage = SentEchoFromWitness
		case messages.ReliableProtocolMessage_ECHO_FROM_WITNESS:
			if !msgState.ownWitnessSet[senderPid] || msgState.stage >= SentEchoFromProcess {
				return
			}
			p.broadcastToWitnesses(
				context,
				&messages.ReliableProtocolMessage{
					Stage:         messages.ReliableProtocolMessage_ECHO_FROM_PROCESS,
					SourceMessage: sourceMessage,
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
				p.broadcastReadyFromWitness(context, sourceMessage, msgState)
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
					&messages.ReliableProtocolMessage{
						Stage:         messages.ReliableProtocolMessage_READY_FROM_PROCESS,
						SourceMessage: sourceMessage,
					},
					msgState)
				msgState.stage = SentReadyFromProcess
			}
		case messages.ReliableProtocolMessage_READY_FROM_PROCESS:
			if !p.isWitness(msgState) ||
				msgState.witnessStage == SentValidate ||
				msgState.readyFromProcesses[senderPid] {
				return
			}

			msgState.readyFromProcesses[senderPid] = true
			msgState.readyFromProcessesStat[value]++

			if msgState.witnessStage < SentReadyFromWitness &&
				msgState.readyFromProcessesStat[value] >= p.readyMessagesThreshold {
				p.broadcastReadyFromWitness(context, sourceMessage, msgState)
			}

			if msgState.readyFromProcessesStat[value] >= p.quorumThreshold {
				p.broadcastProtocolMessage(
					context,
					&messages.ReliableProtocolMessage{
						Stage:         messages.ReliableProtocolMessage_VALIDATE,
						SourceMessage: sourceMessage,
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
				p.deliver(sourceMessage)
			}
		}
	case *messages.RecoveryProtocolMessage:
		msg := message.(*messages.RecoveryProtocolMessage)
		reliableMessage := msg.Message
		sourceMessage := reliableMessage.SourceMessage
		value := protocols.ValueType(sourceMessage.Value)

		sender := context.Sender()
		senderPid := utils.MakeCustomPid(sender)

		p.logger.OnMessageReceived(senderPid, msg.Stamp)

		delivered := p.delivered(sourceMessage)
		recoveryState := p.initRecoveryMessageState(sourceMessage)
		recoveryState.receivedMessagesCnt++

		switch msg.RecoveryStage {
		case messages.RecoveryProtocolMessage_RECOVER:
			if recoveryState.receivedRecover[senderPid] || taggedWithP(reliableMessage) {
				return
			}

			recoveryState.receivedRecover[senderPid] = true
			recoveryState.recoverMessagesStat[value]++
			if reliableMessage.Stage == messages.ReliableProtocolMessage_READY_FROM_PROCESS {
				recoveryState.recoverReadyStat[value]++
			}

			if delivered {
				p.sendRecoveryMessage(
					context,
					sender,
					&messages.RecoveryProtocolMessage{
						RecoveryStage: messages.RecoveryProtocolMessage_REPLY,
						Message:       reliableMessage,
					},
				)
			}
			recoverMessagesCnt := len(recoveryState.receivedRecover)

			if recoveryState.stage < SentRecover &&
				recoverMessagesCnt >= p.readyMessagesThreshold {
				p.broadcastRecover(context, sourceMessage, recoveryState)
			}

			if recoveryState.stage < SentEcho &&
				(recoveryState.recoverReadyStat[value] >= p.readyMessagesThreshold ||
					recoverMessagesCnt >= p.quorumThreshold &&
						recoveryState.recoverMessagesStat[value] == recoverMessagesCnt) {
				p.broadcastEcho(
					context,
					reliableMessage,
					recoveryState,
				)
			}
		case messages.RecoveryProtocolMessage_REPLY:
			if recoveryState.receivedReply[senderPid] {
				return
			}

			recoveryState.receivedReply[senderPid] = true
			recoveryState.replyMessagesStat[value]++

			if !delivered && recoveryState.replyMessagesStat[value] >= p.readyMessagesThreshold {
				p.deliver(sourceMessage)
			}
		case messages.RecoveryProtocolMessage_ECHO:
			if recoveryState.receivedEcho[senderPid] {
				return
			}

			recoveryState.receivedEcho[senderPid] = true
			recoveryState.echoMessagesStat[value]++

			if recoveryState.stage < SentReady &&
				recoveryState.echoMessagesStat[value] >= p.quorumThreshold {
				p.broadcastReady(context, reliableMessage, recoveryState)
			}
		case messages.RecoveryProtocolMessage_READY:
			if recoveryState.receivedReady[senderPid] {
				return
			}

			recoveryState.receivedReady[senderPid] = true
			recoveryState.readyMessagesStat[value]++

			if recoveryState.stage < SentReady &&
				recoveryState.readyMessagesStat[value] >= p.readyMessagesThreshold {
				p.broadcastReady(context, reliableMessage, recoveryState)
			}

			if !delivered && recoveryState.readyMessagesStat[value] >= p.quorumThreshold {
				p.deliver(sourceMessage)
			}
		}
	}
}

func (p *Process) Broadcast(context actor.SenderContext, value int64) {
	sourceMessage := &messages.SourceMessage{
		Author:    p.pid,
		SeqNumber: p.transactionCounter,
		Value:     value,
	}

	p.sendProtocolMessage(
		context,
		p.actorPid,
		&messages.ReliableProtocolMessage{
			Stage:         messages.ReliableProtocolMessage_NOTIFY,
			SourceMessage: sourceMessage,
		})

	p.logger.OnTransactionInit(sourceMessage)

	p.transactionCounter++
}
