package consistent

import (
	"fmt"
	"math"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/utils"
)

type ProcessId int32

type messageState struct {
	receivedEcho map[ProcessId]bool
	echoCount    map[int32]int
	witnessSet   map[string]bool

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)

	ms.receivedEcho = make(map[ProcessId]bool)
	ms.echoCount = make(map[int32]int)

	ms.receivedMessagesCnt = 0

	return ms
}

type Process struct {
	processIndex int32
	actorPids    map[string]ProcessId
	pids         []string

	transactionCounter int32

	deliveredMessages map[ProcessId]map[int32]int32
	messagesLog       map[ProcessId]map[int32]*messageState

	witnessThreshold int

	wSelector   *hashing.WitnessesSelector
	historyHash *hashing.HistoryHash

	logger *eventlogger.EventLogger
	ownDeliveredTransactions chan bool
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

	p.actorPids = make(map[string]ProcessId)
	p.deliveredMessages = make(map[ProcessId]map[int32]int32)
	p.messagesLog = make(map[ProcessId]map[int32]*messageState)

	p.witnessThreshold = parameters.WitnessThreshold

	for i, pid := range actorPids {
		p.actorPids[pid] = ProcessId(i)
		p.deliveredMessages[ProcessId(i)] = make(map[int32]int32)
		p.messagesLog[ProcessId(i)] = make(map[int32]*messageState)
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
	bInstance *messages.BroadcastInstance,
) *messageState {
	msgState := newMessageState()
	p.messagesLog[ProcessId(bInstance.Author)][bInstance.SeqNumber] = msgState

	//p.logger.OnHistoryUsedInWitnessSetSelection(
	//	bInstance,
	//	p.historyHash,
	//	p.deliveredMessagesHistory,
	//)

	msgState.witnessSet, _ =
		p.wSelector.GetWitnessSet(p.pids, bInstance.Author, bInstance.SeqNumber, p.historyHash)

	p.logger.OnWitnessSetSelected("own", bInstance, msgState.witnessSet)

	return msgState
}

func (p *Process) sendMessage(
	reliableContext *context.ReliableContext,
	to ProcessId,
	bInstance *messages.BroadcastInstance,
	message *messages.ConsistentProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_ConsistentProtocolMessage{
			ConsistentProtocolMessage: message.Copy(),
		},
	}

	msg := reliableContext.MakeNewMessage()
	msg.Content = &messages.Message_BroadcastInstanceMessage{
		BroadcastInstanceMessage: bMessage,
	}

	reliableContext.Send(int32(to), msg)
}

func (p *Process) broadcast(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	message *messages.ConsistentProtocolMessage,
) {
	for _, pid := range p.actorPids {
		p.sendMessage(reliableContext, pid, bInstance, message)
	}
}

func (p *Process) deliver(bInstance *messages.BroadcastInstance, value int32) {
	author := ProcessId(bInstance.Author)

	p.deliveredMessages[author][bInstance.SeqNumber] = value
	p.historyHash.Insert(
		utils.TransactionToBytes(p.pids[bInstance.Author], int64(bInstance.SeqNumber)))

	if p.sendOwnDeliveredTransactions && bInstance.Author == p.processIndex {
		p.ownDeliveredTransactions <- true
	}

	messagesReceived :=
		p.messagesLog[author][bInstance.SeqNumber].receivedMessagesCnt

	delete(p.messagesLog[author], bInstance.SeqNumber)
	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *Process) verify(
	reliableContext *context.ReliableContext,
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	value int32,
) bool {
	author := ProcessId(bInstance.Author)
	msgState := p.messagesLog[author][bInstance.SeqNumber]

	deliveredValue, delivered :=
		p.deliveredMessages[author][bInstance.SeqNumber]
	if delivered {
		if deliveredValue != value {
			p.logger.OnAttack(bInstance, value, deliveredValue)
			return false
		}
	} else if msgState != nil {
		msgState.receivedMessagesCnt++
		if msgState.witnessSet[p.pids[senderId]] && !msgState.receivedEcho[senderId] {
			msgState.receivedEcho[senderId] = true
			msgState.echoCount[value]++
			if msgState.echoCount[value] >= p.witnessThreshold {
				p.deliver(bInstance, value)
			}
		}
	} else {
		msgState = p.initMessageState(bInstance)
		msgState.receivedMessagesCnt++

		message := &messages.ConsistentProtocolMessage{
			Stage: messages.ConsistentProtocolMessage_VERIFY,
			Value: value,
		}
		for pid := range msgState.witnessSet {
			p.sendMessage(reliableContext, p.actorPids[pid], bInstance, message)
		}
	}
	return true
}

func (p *Process) HandleMessage(
	reliableContext *context.ReliableContext,
	sender int32,
	broadcastInstanceMessage *messages.BroadcastInstanceMessage,
) {
	bInstance := broadcastInstanceMessage.BroadcastInstance

	switch protocolMessage := broadcastInstanceMessage.Message.(type) {
	case *messages.BroadcastInstanceMessage_ConsistentProtocolMessage:
		consistentMessage := protocolMessage.ConsistentProtocolMessage

		doBroadcast := p.verify(
			reliableContext,
			ProcessId(sender),
			bInstance,
			consistentMessage.Value)

		if consistentMessage.Stage == messages.ConsistentProtocolMessage_VERIFY && doBroadcast {
			p.broadcast(
				reliableContext,
				bInstance,
				&messages.ConsistentProtocolMessage{
					Stage: messages.ConsistentProtocolMessage_ECHO,
					Value: consistentMessage.Value,
				})
		}
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

	p.verify(reliableContext, ProcessId(p.processIndex), broadcastInstance, value)

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
