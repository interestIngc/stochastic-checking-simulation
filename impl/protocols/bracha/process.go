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
	"stochastic-checking-simulation/impl/utils"
)

type Stage int

const (
	Init Stage = iota
	SentEcho
	SentReady
)

type messageState struct {
	receivedEcho  map[string]bool
	receivedReady map[string]bool

	echoCount  map[int64]int
	readyCount map[int64]int

	stage Stage

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)
	ms.receivedEcho = make(map[string]bool)
	ms.receivedReady = make(map[string]bool)
	ms.echoCount = make(map[int64]int)
	ms.readyCount = make(map[int64]int)

	ms.stage = Init

	ms.receivedMessagesCnt = 0

	return ms
}

type Process struct {
	actorPid           *actor.PID
	pid                string
	actorPids          map[string]*actor.PID
	transactionCounter int64
	messageCounter     int64

	deliveredMessages map[string]map[int64]int64
	messagesLog       map[string]map[int64]*messageState

	messagesForEcho     int
	messagesForReady    int
	messagesForDelivery int

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

	p.messagesForEcho = int(math.Ceil(float64(len(actorPids)+parameters.FaultyProcesses+1) / float64(2)))
	p.messagesForReady = parameters.FaultyProcesses + 1
	p.messagesForDelivery = 2*parameters.FaultyProcesses + 1

	p.deliveredMessages = make(map[string]map[int64]int64)
	p.messagesLog = make(map[string]map[int64]*messageState)
	for _, currActorPid := range actorPids {
		pid := utils.MakeCustomPid(currActorPid)
		p.actorPids[pid] = currActorPid
		p.deliveredMessages[pid] = make(map[int64]int64)
		p.messagesLog[pid] = make(map[int64]*messageState)
	}

	p.logger = eventlogger.InitEventLogger(p.pid, logger)
	p.transactionManager = transactionManager
}

func (p *Process) initMessageState(bInstance *messages.BroadcastInstance) *messageState {
	msgState := p.messagesLog[bInstance.Author][bInstance.SeqNumber]
	if msgState == nil {
		msgState = newMessageState()
		p.messagesLog[bInstance.Author][bInstance.SeqNumber] = msgState
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
		Message: &messages.BroadcastInstanceMessage_BrachaProtocolMessage{
			BrachaProtocolMessage: message.Copy(),
		},
		Stamp: p.messageCounter,
	}

	context.RequestWithCustomSender(to, bMessage, p.actorPid)
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
	messagesReceived :=
		p.messagesLog[bInstance.Author][bInstance.SeqNumber].receivedMessagesCnt

	delete(p.messagesLog[bInstance.Author], bInstance.SeqNumber)
	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *Process) processProtocolMessage(
	context actor.Context,
	senderPid string,
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
	case *messages.Simulate:
		p.transactionManager.Simulate(context, p)
	case *messages.BroadcastInstanceMessage:
		bInstance := message.BroadcastInstance

		senderPid := utils.MakeCustomPid(context.Sender())
		p.logger.OnMessageReceived(senderPid, message.Stamp)

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
		Author:    p.pid,
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
