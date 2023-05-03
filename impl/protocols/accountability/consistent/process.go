package consistent

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"math"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/utils"
)

type ProcessId int64

type messageState struct {
	receivedEcho map[ProcessId]bool
	echoCount    map[int64]int
	witnessSet   map[string]bool

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)

	ms.receivedEcho = make(map[ProcessId]bool)
	ms.echoCount = make(map[int64]int)

	ms.receivedMessagesCnt = 0

	return ms
}

type CorrectProcess struct {
	processIndex int64
	actorPids    map[string]*actor.PID
	pids         []string

	transactionCounter int64

	deliveredMessages map[ProcessId]map[int64]int64
	messagesLog       map[ProcessId]map[int64]*messageState

	witnessThreshold int

	wSelector   *hashing.WitnessesSelector
	historyHash *hashing.HistoryHash

	logger *eventlogger.EventLogger
}

func (p *CorrectProcess) InitProcess(
	processIndex int64,
	actorPids []*actor.PID,
	parameters *parameters.Parameters,
	logger *eventlogger.EventLogger,
) {
	p.processIndex = processIndex
	p.actorPids = make(map[string]*actor.PID)
	p.pids = make([]string, len(actorPids))

	p.transactionCounter = 0

	p.deliveredMessages = make(map[ProcessId]map[int64]int64)
	p.messagesLog = make(map[ProcessId]map[int64]*messageState)

	p.witnessThreshold = parameters.WitnessThreshold

	for i, actorPid := range actorPids {
		pid := utils.MakeCustomPid(actorPid)
		p.pids[i] = pid
		p.actorPids[pid] = actorPid
		p.deliveredMessages[ProcessId(i)] = make(map[int64]int64)
		p.messagesLog[ProcessId(i)] = make(map[int64]*messageState)
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
}

func (p *CorrectProcess) initMessageState(
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

func (p *CorrectProcess) sendMessage(
	actorContext actor.Context,
	reliableContext *context.ReliableContext,
	to *actor.PID,
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

	reliableContext.Send(actorContext, to, msg)
}

func (p *CorrectProcess) broadcast(
	actorContext actor.Context,
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	message *messages.ConsistentProtocolMessage) {
	for _, pid := range p.actorPids {
		p.sendMessage(actorContext, reliableContext, pid, bInstance, message)
	}
}

func (p *CorrectProcess) deliver(bInstance *messages.BroadcastInstance, value int64) {
	author := ProcessId(bInstance.Author)

	p.deliveredMessages[author][bInstance.SeqNumber] = value
	p.historyHash.Insert(
		utils.TransactionToBytes(p.pids[bInstance.Author], bInstance.SeqNumber))

	messagesReceived :=
		p.messagesLog[author][bInstance.SeqNumber].receivedMessagesCnt

	delete(p.messagesLog[author], bInstance.SeqNumber)
	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *CorrectProcess) verify(
	actorContext actor.Context,
	reliableContext *context.ReliableContext,
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	value int64,
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
			p.sendMessage(actorContext, reliableContext, p.actorPids[pid], bInstance, message)
		}
	}
	return true
}

func (p *CorrectProcess) HandleMessage(
	actorContext actor.Context,
	reliableContext *context.ReliableContext,
	sender int64,
	broadcastInstanceMessage *messages.BroadcastInstanceMessage,
) {
	bInstance := broadcastInstanceMessage.BroadcastInstance

	switch protocolMessage := broadcastInstanceMessage.Message.(type) {
	case *messages.BroadcastInstanceMessage_ConsistentProtocolMessage:
		consistentMessage := protocolMessage.ConsistentProtocolMessage

		doBroadcast := p.verify(
			actorContext,
			reliableContext,
			ProcessId(sender),
			bInstance,
			consistentMessage.Value)

		if consistentMessage.Stage == messages.ConsistentProtocolMessage_VERIFY && doBroadcast {
			p.broadcast(
				actorContext,
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

func (p *CorrectProcess) Broadcast(
	actorContext actor.Context,
	reliableContext *context.ReliableContext,
	value int64,
) {
	broadcastInstance := &messages.BroadcastInstance{
		Author:    p.processIndex,
		SeqNumber: p.transactionCounter,
	}

	p.verify(actorContext, reliableContext, ProcessId(p.processIndex), broadcastInstance, value)

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
