package bracha

import (
	"fmt"
	"math"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
)

type Stage int

const (
	Init Stage = iota
	SentEcho
	SentReady
)

type ProcessId int32

type messageState struct {
	receivedEcho  map[ProcessId]bool
	receivedReady map[ProcessId]bool

	echoCount  map[int32]int
	readyCount map[int32]int

	stage Stage

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)
	ms.receivedEcho = make(map[ProcessId]bool)
	ms.receivedReady = make(map[ProcessId]bool)
	ms.echoCount = make(map[int32]int)
	ms.readyCount = make(map[int32]int)

	ms.stage = Init

	ms.receivedMessagesCnt = 0

	return ms
}

type Process struct {
	processIndex int32

	transactionCounter int32

	deliveredTransactions map[ProcessId]map[int32]int32
	transactionsLog       map[ProcessId]map[int32]*messageState

	n                   int
	messagesForEcho     int
	messagesForReady    int
	messagesForDelivery int

	logger *eventlogger.EventLogger
	ownDeliveredTransactions chan bool
}

func (p *Process) InitProcess(
	processIndex int32,
	actorPids []string,
	parameters *parameters.Parameters,
	logger *eventlogger.EventLogger,
	ownDeliveredTransactions chan bool,
) {
	p.processIndex = processIndex
	p.n = len(actorPids)
	f := parameters.FaultyProcesses

	p.transactionCounter = 0

	p.messagesForEcho = int(math.Ceil(float64(p.n+f+1) / float64(2)))
	p.messagesForReady = f + 1
	p.messagesForDelivery = 2*f + 1

	p.deliveredTransactions = make(map[ProcessId]map[int32]int32)
	p.transactionsLog = make(map[ProcessId]map[int32]*messageState)
	for index := 0; index < p.n; index++ {
		p.deliveredTransactions[ProcessId(index)] = make(map[int32]int32)
		p.transactionsLog[ProcessId(index)] = make(map[int32]*messageState)
	}

	p.logger = logger
	p.ownDeliveredTransactions = ownDeliveredTransactions
}

func (p *Process) initMessageState(bInstance *messages.BroadcastInstance) *messageState {
	author := ProcessId(bInstance.Author)
	msgState := p.transactionsLog[author][bInstance.SeqNumber]
	if msgState == nil {
		msgState = newMessageState()
		p.transactionsLog[author][bInstance.SeqNumber] = msgState
	}
	return msgState
}

func (p *Process) sendProtocolMessage(
	reliableContext *context.ReliableContext,
	to ProcessId,
	bInstance *messages.BroadcastInstance,
	message *messages.BrachaProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_BrachaProtocolMessage{
			BrachaProtocolMessage: message.Copy(),
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
	message *messages.BrachaProtocolMessage,
) {
	for i := 0; i < p.n; i++ {
		p.sendProtocolMessage(reliableContext, ProcessId(i), bInstance, message)
	}
}

func (p *Process) broadcastEcho(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	value int32,
	msgState *messageState,
) {
	p.broadcast(
		reliableContext,
		bInstance,
		&messages.BrachaProtocolMessage{
			Stage: messages.BrachaProtocolMessage_ECHO,
			Value: value,
		})
	msgState.stage = SentEcho
}

func (p *Process) broadcastReady(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	value int32,
	msgState *messageState,
) {
	p.broadcast(
		reliableContext,
		bInstance,
		&messages.BrachaProtocolMessage{
			Stage: messages.BrachaProtocolMessage_READY,
			Value: value,
		})
	msgState.stage = SentReady
}

func (p *Process) delivered(
	bInstance *messages.BroadcastInstance,
	value int32,
) bool {
	deliveredValue, delivered :=
		p.deliveredTransactions[ProcessId(bInstance.Author)][bInstance.SeqNumber]

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
	p.deliveredTransactions[author][bInstance.SeqNumber] = value
	messagesReceived :=
		p.transactionsLog[author][bInstance.SeqNumber].receivedMessagesCnt

	if bInstance.Author == p.processIndex {
		p.ownDeliveredTransactions <- true
	}

	delete(p.transactionsLog[author], bInstance.SeqNumber)
	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *Process) processProtocolMessage(
	reliableContext *context.ReliableContext,
	senderPid ProcessId,
	bInstance *messages.BroadcastInstance,
	message *messages.BrachaProtocolMessage,
) {
	value := message.Value

	if p.delivered(bInstance, value) {
		return
	}

	msgState := p.initMessageState(bInstance)
	msgState.receivedMessagesCnt++

	switch message.Stage {
	case messages.BrachaProtocolMessage_INITIAL:
		if msgState.stage == Init {
			p.broadcastEcho(reliableContext, bInstance, value, msgState)
		}
	case messages.BrachaProtocolMessage_ECHO:
		if msgState.stage == SentReady || msgState.receivedEcho[senderPid] {
			return
		}
		msgState.receivedEcho[senderPid] = true
		msgState.echoCount[value]++

		if msgState.echoCount[value] >= p.messagesForEcho {
			if msgState.stage == Init {
				p.broadcastEcho(reliableContext, bInstance, value, msgState)
			}
			if msgState.stage == SentEcho {
				p.broadcastReady(reliableContext, bInstance, value, msgState)
			}
		}
	case messages.BrachaProtocolMessage_READY:
		if msgState.receivedReady[senderPid] {
			return
		}
		msgState.receivedReady[senderPid] = true
		msgState.readyCount[value]++

		if msgState.readyCount[value] >= p.messagesForReady {
			if msgState.stage == Init {
				p.broadcastEcho(reliableContext, bInstance, value, msgState)
			}
			if msgState.stage == SentEcho {
				p.broadcastReady(reliableContext, bInstance, value, msgState)
			}
		}
		if msgState.readyCount[value] >= p.messagesForDelivery {
			p.deliver(bInstance, value)
		}
	}
}

func (p *Process) HandleMessage(
	reliableContext *context.ReliableContext,
	sender int32,
	broadcastInstanceMessage *messages.BroadcastInstanceMessage,
) {
	bInstance := broadcastInstanceMessage.BroadcastInstance

	switch protocolMessage := broadcastInstanceMessage.Message.(type) {
	case *messages.BroadcastInstanceMessage_BrachaProtocolMessage:
		p.processProtocolMessage(
			reliableContext,
			ProcessId(sender),
			bInstance,
			protocolMessage.BrachaProtocolMessage,
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

	p.broadcast(
		reliableContext,
		broadcastInstance,
		&messages.BrachaProtocolMessage{
			Stage: messages.BrachaProtocolMessage_INITIAL,
			Value: value,
		})

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
