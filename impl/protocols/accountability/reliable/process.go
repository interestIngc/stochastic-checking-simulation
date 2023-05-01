package reliable

import (
	"fmt"
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

type ProcessId int64

type WitnessStage int

const (
	InitialWitnessStage WitnessStage = iota
	SentEchoFromWitness
	SentReadyFromWitness
	SentValidate
)

type Stage int

const (
	InitialStage Stage = iota
	SentEchoFromProcess
	SentReadyFromProcess
)

type RecoveryStage int

const (
	InitialRecoveryStage RecoveryStage = iota
	SentRecover
	SentEcho
	SentReady
)

type messageState struct {
	echoFromProcesses     map[ProcessId]bool
	readyFromProcesses    map[ProcessId]bool
	readyFromWitnesses    map[ProcessId]bool
	validateFromWitnesses map[ProcessId]bool

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

	ms.echoFromProcesses = make(map[ProcessId]bool)
	ms.readyFromProcesses = make(map[ProcessId]bool)
	ms.readyFromWitnesses = make(map[ProcessId]bool)
	ms.validateFromWitnesses = make(map[ProcessId]bool)

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
	receivedRecover map[ProcessId]bool
	receivedReply   map[ProcessId]bool
	receivedEcho    map[ProcessId]bool
	receivedReady   map[ProcessId]bool

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

	ms.receivedRecover = make(map[ProcessId]bool)
	ms.receivedReply = make(map[ProcessId]bool)
	ms.receivedEcho = make(map[ProcessId]bool)
	ms.receivedReady = make(map[ProcessId]bool)

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
	processIndex int64
	actorPids    map[string]*actor.PID
	pids         []string

	transactionCounter int64
	messageCounter     int64

	deliveredMessages   map[ProcessId]map[int64]int64
	messagesLog         map[ProcessId]map[int64]*messageState
	lastSentPMessages   map[ProcessId]map[int64]*messages.ReliableProtocolMessage
	recoveryMessagesLog map[ProcessId]map[int64]*recoveryMessageState

	quorumThreshold         int
	readyMessagesThreshold  int
	recoverySwitchTimeoutNs time.Duration
	witnessThreshold        int

	wSelector   *hashing.WitnessesSelector
	historyHash *hashing.HistoryHash

	logger             *eventlogger.EventLogger
	transactionManager *protocols.TransactionManager

	mainServer *actor.PID
}

func (p *Process) InitProcess(
	processIndex int64,
	actorPids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger,
	transactionManager *protocols.TransactionManager,
	mainServer *actor.PID,
) {
	p.processIndex = processIndex
	p.actorPids = make(map[string]*actor.PID)
	p.pids = make([]string, len(actorPids))

	p.transactionCounter = 0
	p.messageCounter = 0

	p.quorumThreshold = int(math.Ceil(float64(len(actorPids)+parameters.FaultyProcesses+1) / float64(2)))
	p.readyMessagesThreshold = parameters.FaultyProcesses + 1
	p.recoverySwitchTimeoutNs = time.Duration(parameters.RecoverySwitchTimeoutNs)
	p.witnessThreshold = parameters.WitnessThreshold

	p.deliveredMessages = make(map[ProcessId]map[int64]int64)
	p.messagesLog = make(map[ProcessId]map[int64]*messageState)
	//p.lastSentPMessages = make(map[ProcessId]map[int64]*messages.ReliableProtocolMessage)
	//p.recoveryMessagesLog = make(map[ProcessId]map[int64]*recoveryMessageState)

	for i, actorPid := range actorPids {
		pid := utils.MakeCustomPid(actorPid)
		p.pids[i] = pid
		p.actorPids[pid] = actorPid
		p.deliveredMessages[ProcessId(i)] = make(map[int64]int64)
		p.messagesLog[ProcessId(i)] = make(map[int64]*messageState)
		//p.lastSentPMessages[ProcessId(i)] = make(map[int64]*messages.ReliableProtocolMessage)
		//p.recoveryMessagesLog[ProcessId(i)] = make(map[int64]*recoveryMessageState)
	}

	var hasher hashing.Hasher
	if parameters.NodeIdSize == 256 {
		hasher = hashing.HashSHA256{}
	} else {
		hasher = hashing.HashSHA512{}
	}

	p.wSelector = &hashing.WitnessesSelector{
		Hasher:               hasher,
		MinPotWitnessSetSize: parameters.MinPotWitnessSetSize,
		MinOwnWitnessSetSize: parameters.MinOwnWitnessSetSize,
		PotWitnessSetRadius:  parameters.PotWitnessSetRadius,
		OwnWitnessSetRadius:  parameters.OwnWitnessSetRadius,
	}
	binCapacity := uint(math.Pow(2, float64(parameters.NodeIdSize/parameters.NumberOfBins)))
	p.historyHash = hashing.NewHistoryHash(uint(parameters.NumberOfBins), binCapacity, hasher)

	p.logger = eventlogger.InitEventLogger(p.processIndex, logger)
	p.transactionManager = transactionManager

	p.mainServer = mainServer
}

func (p *Process) initMessageState(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	value int64,
) *messageState {
	msgState := newMessageState()
	p.messagesLog[ProcessId(bInstance.Author)][bInstance.SeqNumber] = msgState

	msgState.ownWitnessSet, msgState.potWitnessSet =
		p.wSelector.GetWitnessSet(p.pids, bInstance.Author, bInstance.SeqNumber, p.historyHash)

	p.logger.OnWitnessSetSelected("own", bInstance, msgState.ownWitnessSet)
	p.logger.OnWitnessSetSelected("pot", bInstance, msgState.potWitnessSet)

	p.broadcastToWitnesses(
		context,
		bInstance,
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
	msgState := p.messagesLog[ProcessId(bInstance.Author)][bInstance.SeqNumber]
	if msgState == nil {
		msgState = p.initMessageState(context, bInstance, value)
		//context.ReenterAfter(
		//	actor.NewFuture(context.ActorSystem(), p.recoverySwitchTimeoutNs),
		//	func(res interface{}, err error) {
		//		if !p.delivered(bInstance, value) {
		//			p.logger.OnRecoveryProtocolSwitch(bInstance)
		//			recoveryState := p.initRecoveryMessageState(bInstance)
		//			p.broadcastRecover(context, bInstance, recoveryState)
		//		}
		//	})
	}
	return msgState
}

func (p *Process) initRecoveryMessageState(
	bInstance *messages.BroadcastInstance,
) *recoveryMessageState {
	author := ProcessId(bInstance.Author)
	recoveryState := p.recoveryMessagesLog[author][bInstance.SeqNumber]

	if recoveryState == nil {
		recoveryState = newRecoveryMessageState()
		p.recoveryMessagesLog[author][bInstance.SeqNumber] = recoveryState
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
		Sender:            p.processIndex,
		Message: &messages.BroadcastInstanceMessage_ReliableProtocolMessage{
			ReliableProtocolMessage: reliableMessage.Copy(),
		},
		Stamp: p.messageCounter,
	}

	context.Send(to, bMessage)
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
		Sender:            p.processIndex,
		Message: &messages.BroadcastInstanceMessage_RecoveryProtocolMessage{
			RecoveryProtocolMessage: recoveryMessage.Copy(),
		},
		Stamp: p.messageCounter,
	}

	context.Send(to, bMessage)
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

	//p.lastSentPMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber] = message
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
	lastProcessMessage := p.lastSentPMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber]

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
	return msgState.potWitnessSet[p.pids[p.processIndex]]
}

func (p *Process) delivered(
	bInstance *messages.BroadcastInstance,
	value int64,
) bool {
	deliveredValue, delivered :=
		p.deliveredMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber]

	if delivered && deliveredValue != value {
		p.logger.OnAttack(bInstance, value, deliveredValue)
	}

	return delivered
}

func (p *Process) deliver(
	bInstance *messages.BroadcastInstance,
	value int64,
) {
	author := ProcessId(bInstance.Author)
	p.deliveredMessages[author][bInstance.SeqNumber] = value
	p.historyHash.Insert(
		utils.TransactionToBytes(p.pids[bInstance.Author], bInstance.SeqNumber))

	msgState := p.messagesLog[author][bInstance.SeqNumber]
	messagesReceived := msgState.receivedMessagesCnt
	delete(p.messagesLog[author], bInstance.SeqNumber)

	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *Process) processReliableProtocolMessage(
	context actor.Context,
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
) {
	value := reliableMessage.Value

	if p.delivered(bInstance, value) {
		return
	}

	msgState := p.registerMessage(context, bInstance, value)
	msgState.receivedMessagesCnt++

	senderPid := p.pids[senderId]

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
			msgState.echoFromProcesses[senderId] {
			return
		}

		msgState.echoFromProcesses[senderId] = true
		msgState.echoFromProcessesStat[value]++

		if msgState.echoFromProcessesStat[value] >= p.quorumThreshold {
			p.broadcastReadyFromWitness(context, bInstance, value, msgState)
		}
	case messages.ReliableProtocolMessage_READY_FROM_WITNESS:
		if !msgState.ownWitnessSet[senderPid] ||
			msgState.stage >= SentReadyFromProcess ||
			msgState.readyFromWitnesses[senderId] {
			return
		}

		msgState.readyFromWitnesses[senderId] = true
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
		if !p.isWitness(msgState) || msgState.readyFromProcesses[senderId] {
			return
		}

		msgState.readyFromProcesses[senderId] = true
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
		if !msgState.ownWitnessSet[senderPid] || msgState.validateFromWitnesses[senderId] {
			return
		}

		msgState.validateFromWitnesses[senderId] = true
		msgState.validatesStat[value]++

		if msgState.validatesStat[value] >= p.witnessThreshold {
			//recoveryState := p.recoveryMessagesLog[ProcessId(bInstance.Author)][bInstance.SeqNumber]
			//
			//if recoveryState != nil {
			//	for pid := range recoveryState.receivedRecover {
			//		p.sendRecoveryMessage(
			//			context,
			//			p.actorPids[p.pids[pid]],
			//			bInstance,
			//			&messages.RecoveryProtocolMessage{
			//				Stage:                   messages.RecoveryProtocolMessage_REPLY,
			//				ReliableProtocolMessage: reliableMessage,
			//			})
			//	}
			//}

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
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	recoveryMessage *messages.RecoveryProtocolMessage,
) {
	reliableMessage := recoveryMessage.ReliableProtocolMessage

	deliveredValue, delivered :=
		p.deliveredMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber]

	recoveryState := p.initRecoveryMessageState(bInstance)
	recoveryState.receivedMessagesCnt++

	switch recoveryMessage.Stage {
	case messages.RecoveryProtocolMessage_RECOVER:
		if recoveryState.receivedRecover[senderId] {
			return
		}

		recoveryState.receivedRecover[senderId] = true
		recoverMessagesCnt := len(recoveryState.receivedRecover)

		if delivered {
			p.sendRecoveryMessage(
				context,
				p.actorPids[p.pids[senderId]],
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
		if recoveryState.receivedReply[senderId] || reliableMessage == nil {
			return
		}

		recoveryState.receivedReply[senderId] = true
		recoveryState.replyMessagesStat[reliableMessage.Value]++

		if !delivered &&
			recoveryState.replyMessagesStat[reliableMessage.Value] >= p.readyMessagesThreshold {
			p.deliver(bInstance, reliableMessage.Value)
		}
	case messages.RecoveryProtocolMessage_ECHO:
		if recoveryState.receivedEcho[senderId] || reliableMessage == nil {
			return
		}

		recoveryState.receivedEcho[senderId] = true
		recoveryState.echoMessagesStat[reliableMessage.Value]++

		if recoveryState.stage < SentReady &&
			recoveryState.echoMessagesStat[reliableMessage.Value] >= p.quorumThreshold {
			p.broadcastReady(context, bInstance, reliableMessage, recoveryState)
		}
	case messages.RecoveryProtocolMessage_READY:
		if recoveryState.receivedReady[senderId] || reliableMessage == nil {
			return
		}

		recoveryState.receivedReady[senderId] = true
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
	switch message := context.Message().(type) {
	case *actor.Started:
		p.logger.OnStart()
		context.Send(
			p.mainServer,
			&messages.Started{Sender: p.processIndex},
		)
	case *actor.Stop:
		p.logger.OnStop()
	case *messages.Simulate:
		p.logger.OnSimulationStart()
		p.transactionManager.Simulate(context, p)
	case *messages.BroadcastInstanceMessage:
		bInstance := message.BroadcastInstance

		p.logger.OnMessageReceived(message.Sender, message.Stamp)
		senderId := ProcessId(message.Sender)

		switch protocolMessage := message.Message.(type) {
		case *messages.BroadcastInstanceMessage_ReliableProtocolMessage:
			p.processReliableProtocolMessage(
				context,
				senderId,
				bInstance,
				protocolMessage.ReliableProtocolMessage,
			)
		case *messages.BroadcastInstanceMessage_RecoveryProtocolMessage:
			p.processRecoveryProtocolMessage(
				context,
				senderId,
				bInstance,
				protocolMessage.RecoveryProtocolMessage,
			)
		default:
			p.logger.Fatal(fmt.Sprintf("Invalid protocol message type %t", protocolMessage))
		}
	}
}

func (p *Process) Broadcast(context actor.SenderContext, value int64) {
	broadcastInstance := &messages.BroadcastInstance{
		Author:    p.processIndex,
		SeqNumber: p.transactionCounter,
	}

	p.sendProtocolMessage(
		context,
		p.actorPids[p.pids[p.processIndex]],
		broadcastInstance,
		&messages.ReliableProtocolMessage{
			Stage: messages.ReliableProtocolMessage_NOTIFY,
			Value: value,
		})

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
