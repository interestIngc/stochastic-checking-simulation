package reliable

import (
	"fmt"
	"math"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/utils"
	"time"
)

type ProcessId int32

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

	echoFromProcessesStat  map[int32]int
	readyFromProcessesStat map[int32]int
	readyFromWitnessesStat map[int32]int
	validatesStat          map[int32]int

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

	ms.echoFromProcessesStat = make(map[int32]int)
	ms.readyFromWitnessesStat = make(map[int32]int)
	ms.readyFromProcessesStat = make(map[int32]int)
	ms.validatesStat = make(map[int32]int)

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

	recoverValues map[int32]bool

	replyMessagesStat map[int32]int
	recoverReadyStat  map[int32]int
	echoMessagesStat  map[int32]int
	readyMessagesStat map[int32]int

	stage RecoveryStage

	receivedMessagesCnt int
}

func newRecoveryMessageState() *recoveryMessageState {
	ms := new(recoveryMessageState)

	ms.receivedRecover = make(map[ProcessId]bool)
	ms.receivedReply = make(map[ProcessId]bool)
	ms.receivedEcho = make(map[ProcessId]bool)
	ms.receivedReady = make(map[ProcessId]bool)

	ms.recoverValues = make(map[int32]bool)

	ms.replyMessagesStat = make(map[int32]int)
	ms.recoverReadyStat = make(map[int32]int)
	ms.echoMessagesStat = make(map[int32]int)
	ms.readyMessagesStat = make(map[int32]int)

	ms.stage = InitialRecoveryStage

	ms.receivedMessagesCnt = 0

	return ms
}

type Process struct {
	processIndex int32
	actorPids    map[string]ProcessId
	pids         []string

	transactionCounter int32

	deliveredMessages   map[ProcessId]map[int32]int32
	messagesLog         map[ProcessId]map[int32]*messageState
	lastSentPMessages   map[ProcessId]map[int32]*messages.ReliableProtocolMessage
	recoveryMessagesLog map[ProcessId]map[int32]*recoveryMessageState

	quorumThreshold         int
	readyMessagesThreshold  int
	recoverySwitchTimeoutNs time.Duration
	witnessThreshold        int

	wSelector   *hashing.WitnessesSelector
	historyHash *hashing.HistoryHash

	logger                       *eventlogger.EventLogger
	ownDeliveredTransactions     chan bool
	sendOwnDeliveredTransactions bool
}

func (p *Process) InitProcess(
	processIndex int32,
	actorPids []string,
	parameters *parameters.Parameters,
	logger *eventlogger.EventLogger,
	ownDeliveredTransactions chan bool,
	sendOwnDeliveredTransactions bool,
) {
	p.processIndex = processIndex
	p.pids = actorPids

	p.transactionCounter = 0

	p.quorumThreshold = int(math.Ceil(float64(len(actorPids)+parameters.FaultyProcesses+1) / float64(2)))
	p.readyMessagesThreshold = parameters.FaultyProcesses + 1
	p.recoverySwitchTimeoutNs = time.Duration(parameters.RecoverySwitchTimeoutNs)
	p.witnessThreshold = parameters.WitnessThreshold

	p.actorPids = make(map[string]ProcessId)
	p.deliveredMessages = make(map[ProcessId]map[int32]int32)
	p.messagesLog = make(map[ProcessId]map[int32]*messageState)
	//p.lastSentPMessages = make(map[ProcessId]map[int32]*messages.ReliableProtocolMessage)
	//p.recoveryMessagesLog = make(map[ProcessId]map[int32]*recoveryMessageState)

	for i, pid := range actorPids {
		p.actorPids[pid] = ProcessId(i)
		p.deliveredMessages[ProcessId(i)] = make(map[int32]int32)
		p.messagesLog[ProcessId(i)] = make(map[int32]*messageState)
		//p.lastSentPMessages[ProcessId(i)] = make(map[int32]*messages.ReliableProtocolMessage)
		//p.recoveryMessagesLog[ProcessId(i)] = make(map[int32]*recoveryMessageState)
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

	p.logger = logger
	p.ownDeliveredTransactions = ownDeliveredTransactions
	p.sendOwnDeliveredTransactions = sendOwnDeliveredTransactions
}

func (p *Process) initMessageState(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	value int32,
) *messageState {
	msgState := newMessageState()
	p.messagesLog[ProcessId(bInstance.Author)][bInstance.SeqNumber] = msgState

	msgState.ownWitnessSet, msgState.potWitnessSet =
		p.wSelector.GetWitnessSet(p.pids, bInstance.Author, bInstance.SeqNumber, p.historyHash)

	p.logger.OnWitnessSetSelected("own", bInstance, msgState.ownWitnessSet)
	p.logger.OnWitnessSetSelected("pot", bInstance, msgState.potWitnessSet)

	p.broadcastToWitnesses(
		reliableContext,
		bInstance,
		&messages.ReliableProtocolMessage{
			Stage: messages.ReliableProtocolMessage_NOTIFY,
			Value: value,
		},
		msgState)

	return msgState
}

func (p *Process) registerMessage(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	value int32,
) *messageState {
	msgState := p.messagesLog[ProcessId(bInstance.Author)][bInstance.SeqNumber]
	if msgState == nil {
		msgState = p.initMessageState(reliableContext, bInstance, value)
		//actorContext.ReenterAfter(
		//	actor.NewFuture(actorContext.ActorSystem(), p.recoverySwitchTimeoutNs),
		//	func(res interface{}, err error) {
		//		if !p.delivered(bInstance, value) {
		//			p.logger.OnRecoveryProtocolSwitch(bInstance)
		//			recoveryState := p.initRecoveryMessageState(bInstance)
		//			p.broadcastRecover(actorContext, bInstance, recoveryState)
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
	reliableContext *context.ReliableContext,
	to ProcessId,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_ReliableProtocolMessage{
			ReliableProtocolMessage: reliableMessage.Copy(),
		},
	}

	msg := reliableContext.MakeNewMessage()
	msg.Content = &messages.Message_BroadcastInstanceMessage{
		BroadcastInstanceMessage: bMessage,
	}

	reliableContext.Send(int32(to), msg)
}

func (p *Process) sendRecoveryMessage(
	reliableContext *context.ReliableContext,
	to ProcessId,
	bInstance *messages.BroadcastInstance,
	recoveryMessage *messages.RecoveryProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_RecoveryProtocolMessage{
			RecoveryProtocolMessage: recoveryMessage.Copy(),
		},
	}

	msg := reliableContext.MakeNewMessage()
	msg.Content = &messages.Message_BroadcastInstanceMessage{
		BroadcastInstanceMessage: bMessage,
	}

	reliableContext.Send(int32(to), msg)
}

func (p *Process) broadcastProtocolMessage(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	message *messages.ReliableProtocolMessage,
) {
	for i := range p.pids {
		p.sendProtocolMessage(reliableContext, ProcessId(i), bInstance, message)
	}
}

func (p *Process) broadcastRecoveryMessage(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	message *messages.RecoveryProtocolMessage,
) {
	for i := range p.pids {
		p.sendRecoveryMessage(reliableContext, ProcessId(i), bInstance, message)
	}
}

func (p *Process) broadcastToWitnesses(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	message *messages.ReliableProtocolMessage,
	msgState *messageState,
) {
	for pid := range msgState.potWitnessSet {
		p.sendProtocolMessage(reliableContext, p.actorPids[pid], bInstance, message)
	}

	//p.lastSentPMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber] = message
}

func (p *Process) broadcastReadyFromWitness(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	value int32,
	msgState *messageState,
) {
	p.broadcastProtocolMessage(
		reliableContext,
		bInstance,
		&messages.ReliableProtocolMessage{
			Stage: messages.ReliableProtocolMessage_READY_FROM_WITNESS,
			Value: value,
		})
	msgState.witnessStage = SentReadyFromWitness
}

func (p *Process) broadcastRecover(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	recoveryState *recoveryMessageState,
) {
	lastProcessMessage := p.lastSentPMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber]

	p.broadcastRecoveryMessage(
		reliableContext,
		bInstance,
		&messages.RecoveryProtocolMessage{
			Stage:                   messages.RecoveryProtocolMessage_RECOVER,
			ReliableProtocolMessage: lastProcessMessage,
		})

	recoveryState.stage = SentRecover
}

func (p *Process) broadcastEcho(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
	recoveryState *recoveryMessageState,
) {
	p.broadcastRecoveryMessage(
		reliableContext,
		bInstance,
		&messages.RecoveryProtocolMessage{
			Stage:                   messages.RecoveryProtocolMessage_ECHO,
			ReliableProtocolMessage: reliableMessage,
		})

	recoveryState.stage = SentEcho
}

func (p *Process) broadcastReady(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
	recoveryState *recoveryMessageState,
) {
	p.broadcastRecoveryMessage(
		reliableContext,
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
	value int32,
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
	value int32,
) {
	author := ProcessId(bInstance.Author)
	p.deliveredMessages[author][bInstance.SeqNumber] = value
	p.historyHash.Insert(
		utils.TransactionToBytes(p.pids[bInstance.Author], int64(bInstance.SeqNumber)))

	if p.sendOwnDeliveredTransactions && bInstance.Author == p.processIndex {
		p.ownDeliveredTransactions <- true
	}

	msgState := p.messagesLog[author][bInstance.SeqNumber]
	messagesReceived := msgState.receivedMessagesCnt
	delete(p.messagesLog[author], bInstance.SeqNumber)

	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *Process) processReliableProtocolMessage(
	reliableContext *context.ReliableContext,
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
) {
	value := reliableMessage.Value

	if p.delivered(bInstance, value) {
		return
	}

	msgState := p.registerMessage(reliableContext, bInstance, value)
	msgState.receivedMessagesCnt++

	senderPid := p.pids[senderId]

	switch reliableMessage.Stage {
	case messages.ReliableProtocolMessage_NOTIFY:
		if !p.isWitness(msgState) || msgState.witnessStage >= SentEchoFromWitness {
			return
		}
		p.broadcastProtocolMessage(
			reliableContext,
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
			reliableContext,
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
			p.broadcastReadyFromWitness(
				reliableContext,
				bInstance,
				value,
				msgState)
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
				reliableContext,
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
			p.broadcastReadyFromWitness(
				reliableContext,
				bInstance,
				value,
				msgState)
		}

		if msgState.witnessStage < SentValidate &&
			msgState.readyFromProcessesStat[value] >= p.quorumThreshold {
			p.broadcastProtocolMessage(
				reliableContext,
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
			//			actorContext,
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

func makeReliableProtocolMessage(value int32) *messages.ReliableProtocolMessage {
	return &messages.ReliableProtocolMessage{
		Stage: messages.ReliableProtocolMessage_NOTIFY,
		Value: value,
	}
}

func (p *Process) processRecoveryProtocolMessage(
	reliableContext *context.ReliableContext,
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
				reliableContext,
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
			p.broadcastRecover(
				reliableContext,
				bInstance,
				recoveryState)
		}

		if reliableMessage != nil {
			recoveryState.recoverValues[reliableMessage.Value] = true
			if reliableMessage.Stage == messages.ReliableProtocolMessage_READY_FROM_PROCESS {
				recoveryState.recoverReadyStat[reliableMessage.Value]++

				if recoveryState.stage < SentEcho &&
					recoveryState.recoverReadyStat[reliableMessage.Value] >= p.readyMessagesThreshold {
					p.broadcastEcho(
						reliableContext,
						bInstance,
						reliableMessage,
						recoveryState)
				}
			}
		}

		if recoveryState.stage < SentEcho &&
			recoverMessagesCnt >= p.quorumThreshold &&
			len(recoveryState.recoverValues) == 1 {
			for value := range recoveryState.recoverValues {
				p.broadcastEcho(
					reliableContext,
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
			p.broadcastReady(
				reliableContext,
				bInstance,
				reliableMessage,
				recoveryState)
		}
	case messages.RecoveryProtocolMessage_READY:
		if recoveryState.receivedReady[senderId] || reliableMessage == nil {
			return
		}

		recoveryState.receivedReady[senderId] = true
		recoveryState.readyMessagesStat[reliableMessage.Value]++

		if recoveryState.stage < SentReady &&
			recoveryState.readyMessagesStat[reliableMessage.Value] >= p.readyMessagesThreshold {
			p.broadcastReady(
				reliableContext,
				bInstance,
				reliableMessage,
				recoveryState)
		}

		if !delivered &&
			recoveryState.readyMessagesStat[reliableMessage.Value] >= p.quorumThreshold {
			p.deliver(bInstance, reliableMessage.Value)
		}
	}
}

func (p *Process) HandleMessage(
	reliableContext *context.ReliableContext,
	sender int32,
	broadcastInstanceMessage *messages.BroadcastInstanceMessage,
) {
	bInstance := broadcastInstanceMessage.BroadcastInstance

	senderId := ProcessId(sender)

	switch protocolMessage := broadcastInstanceMessage.Message.(type) {
	case *messages.BroadcastInstanceMessage_ReliableProtocolMessage:
		p.processReliableProtocolMessage(
			reliableContext,
			senderId,
			bInstance,
			protocolMessage.ReliableProtocolMessage,
		)
	case *messages.BroadcastInstanceMessage_RecoveryProtocolMessage:
		p.processRecoveryProtocolMessage(
			reliableContext,
			senderId,
			bInstance,
			protocolMessage.RecoveryProtocolMessage,
		)
	default:
		p.logger.Fatal(fmt.Sprintf("Invalid protocol message type %t", protocolMessage))
	}
}

func (p *Process) Broadcast(
	reliableContext *context.ReliableContext,
	value int32,
) {
	broadcastInstance := &messages.BroadcastInstance{
		Author:    p.processIndex,
		SeqNumber: p.transactionCounter,
	}

	p.sendProtocolMessage(
		reliableContext,
		p.actorPids[p.pids[p.processIndex]],
		broadcastInstance,
		&messages.ReliableProtocolMessage{
			Stage: messages.ReliableProtocolMessage_NOTIFY,
			Value: value,
		})

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
