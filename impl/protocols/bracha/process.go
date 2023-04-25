package bracha

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"math"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
)

type Stage int

const (
	Init Stage = iota
	SentEcho
	SentReady
)

type ProcessId int64

type messageState struct {
	receivedEcho  map[ProcessId]bool
	receivedReady map[ProcessId]bool

	echoCount  map[int64]int
	readyCount map[int64]int

	stage Stage

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)
	ms.receivedEcho = make(map[ProcessId]bool)
	ms.receivedReady = make(map[ProcessId]bool)
	ms.echoCount = make(map[int64]int)
	ms.readyCount = make(map[int64]int)

	ms.stage = Init

	ms.receivedMessagesCnt = 0

	return ms
}

type Process struct {
	processIndex int64
	actorPids    []*actor.PID

	transactionCounter int64
	messageCounter     int64

	deliveredMessages map[ProcessId]map[int64]int64
	messagesLog       map[ProcessId]map[int64]*messageState

	messagesForEcho     int
	messagesForReady    int
	messagesForDelivery int

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
	p.actorPids = actorPids

	p.transactionCounter = 0
	p.messageCounter = 0

	p.messagesForEcho = int(math.Ceil(float64(len(actorPids)+parameters.FaultyProcesses+1) / float64(2)))
	p.messagesForReady = parameters.FaultyProcesses + 1
	p.messagesForDelivery = 2*parameters.FaultyProcesses + 1

	p.deliveredMessages = make(map[ProcessId]map[int64]int64)
	p.messagesLog = make(map[ProcessId]map[int64]*messageState)
	for index := range actorPids {
		p.deliveredMessages[ProcessId(index)] = make(map[int64]int64)
		p.messagesLog[ProcessId(index)] = make(map[int64]*messageState)
	}

	p.logger = eventlogger.InitEventLogger(p.processIndex, logger)
	p.transactionManager = transactionManager
	p.mainServer = mainServer
}

func (p *Process) initMessageState(bInstance *messages.BroadcastInstance) *messageState {
	author := ProcessId(bInstance.Author)
	msgState := p.messagesLog[author][bInstance.SeqNumber]
	if msgState == nil {
		msgState = newMessageState()
		p.messagesLog[author][bInstance.SeqNumber] = msgState
	}
	return msgState
}

func (p *Process) sendMessage(
	context actor.SenderContext,
	to *actor.PID,
	bInstance *messages.BroadcastInstance,
	message *messages.BrachaProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Sender:            p.processIndex,
		Message: &messages.BroadcastInstanceMessage_BrachaProtocolMessage{
			BrachaProtocolMessage: message.Copy(),
		},
		Stamp: p.messageCounter,
	}

	context.Send(to, bMessage)
	p.logger.OnMessageSent(p.messageCounter)

	p.messageCounter++
}

func (p *Process) broadcast(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	message *messages.BrachaProtocolMessage,
) {
	for _, pid := range p.actorPids {
		p.sendMessage(context, pid, bInstance, message)
	}
}

func (p *Process) broadcastEcho(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	value int64,
	msgState *messageState,
) {
	p.broadcast(
		context,
		bInstance,
		&messages.BrachaProtocolMessage{
			Stage: messages.BrachaProtocolMessage_ECHO,
			Value: value,
		})
	msgState.stage = SentEcho
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	value int64,
	msgState *messageState,
) {
	p.broadcast(
		context,
		bInstance,
		&messages.BrachaProtocolMessage{
			Stage: messages.BrachaProtocolMessage_READY,
			Value: value,
		})
	msgState.stage = SentReady
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
	messagesReceived :=
		p.messagesLog[author][bInstance.SeqNumber].receivedMessagesCnt

	delete(p.messagesLog[author], bInstance.SeqNumber)
	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *Process) processProtocolMessage(
	context actor.Context,
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
			p.broadcastEcho(context, bInstance, value, msgState)
		}
	case messages.BrachaProtocolMessage_ECHO:
		if msgState.stage == SentReady || msgState.receivedEcho[senderPid] {
			return
		}
		msgState.receivedEcho[senderPid] = true
		msgState.echoCount[value]++

		if msgState.echoCount[value] >= p.messagesForEcho {
			if msgState.stage == Init {
				p.broadcastEcho(context, bInstance, value, msgState)
			}
			if msgState.stage == SentEcho {
				p.broadcastReady(context, bInstance, value, msgState)
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
				p.broadcastEcho(context, bInstance, value, msgState)
			}
			if msgState.stage == SentEcho {
				p.broadcastReady(context, bInstance, value, msgState)
			}
		}
		if msgState.readyCount[value] >= p.messagesForDelivery {
			p.deliver(bInstance, value)
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
		p.transactionManager.Simulate(context, p)
	case *messages.BroadcastInstanceMessage:
		bInstance := message.BroadcastInstance

		p.logger.OnMessageReceived(message.Sender, message.Stamp)
		senderPid := ProcessId(message.Sender)

		switch protocolMessage := message.Message.(type) {
		case *messages.BroadcastInstanceMessage_BrachaProtocolMessage:
			p.processProtocolMessage(
				context,
				senderPid,
				bInstance,
				protocolMessage.BrachaProtocolMessage,
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

	p.broadcast(
		context,
		broadcastInstance,
		&messages.BrachaProtocolMessage{
			Stage: messages.BrachaProtocolMessage_INITIAL,
			Value: value,
		})

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
