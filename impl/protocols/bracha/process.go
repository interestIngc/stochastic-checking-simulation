package bracha

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"math"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/manager"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/utils"
)

type Stage int32

const (
	Init Stage = iota
	SentEcho
	SentReady
)

type messageState struct {
	receivedEcho  map[string]bool
	receivedReady map[string]bool

	echoCount  map[protocols.ValueType]int
	readyCount map[protocols.ValueType]int

	stage Stage

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)
	ms.receivedEcho = make(map[string]bool)
	ms.receivedReady = make(map[string]bool)
	ms.echoCount = make(map[protocols.ValueType]int)
	ms.readyCount = make(map[protocols.ValueType]int)

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

	deliveredMessages map[string]map[int64]protocols.ValueType
	messagesLog       map[string]map[int64]*messageState

	messagesForEcho     int
	messagesForReady    int
	messagesForDelivery int

	transactionManager *manager.TransactionManager
	logger             *eventlogger.EventLogger
}

func (p *Process) InitProcess(
	actorPid *actor.PID,
	actorPids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger) {
	p.actorPid = actorPid
	p.pid = utils.MakeCustomPid(actorPid)
	p.actorPids = make(map[string]*actor.PID)

	p.transactionCounter = 0
	p.messageCounter = 0

	p.messagesForEcho = int(math.Ceil(float64(len(actorPids)+parameters.FaultyProcesses+1) / float64(2)))
	p.messagesForReady = parameters.FaultyProcesses + 1
	p.messagesForDelivery = 2*parameters.FaultyProcesses + 1

	p.deliveredMessages = make(map[string]map[int64]protocols.ValueType)
	p.messagesLog = make(map[string]map[int64]*messageState)
	for _, currActorPid := range actorPids {
		pid := utils.MakeCustomPid(currActorPid)
		p.actorPids[pid] = currActorPid
		p.deliveredMessages[pid] = make(map[int64]protocols.ValueType)
		p.messagesLog[pid] = make(map[int64]*messageState)
	}

	p.logger = eventlogger.InitEventLogger(p.pid, logger)
}

func (p *Process) initMessageState(sourceMessage *messages.SourceMessage) *messageState {
	msgState := p.messagesLog[sourceMessage.Author][sourceMessage.SeqNumber]
	if msgState == nil {
		msgState = newMessageState()
		p.messagesLog[sourceMessage.Author][sourceMessage.SeqNumber] = msgState
	}
	return msgState
}

func (p *Process) sendMessage(
	context actor.SenderContext,
	to *actor.PID,
	message *messages.BrachaProtocolMessage) {
	message.Stamp = p.messageCounter

	context.RequestWithCustomSender(to, message.Copy(), p.actorPid)
	p.logger.OnMessageSent(p.messageCounter)

	p.messageCounter++
}

func (p *Process) broadcast(context actor.SenderContext, message *messages.BrachaProtocolMessage) {
	for _, pid := range p.actorPids {
		p.sendMessage(context, pid, message)
	}
}

func (p *Process) broadcastEcho(
	context actor.SenderContext,
	sourceMessage *messages.SourceMessage,
	msgState *messageState) {
	p.broadcast(
		context,
		&messages.BrachaProtocolMessage{
			Stage:         messages.BrachaProtocolMessage_ECHO,
			SourceMessage: sourceMessage,
		})
	msgState.stage = SentEcho
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	sourceMessage *messages.SourceMessage,
	msgState *messageState) {
	p.broadcast(
		context,
		&messages.BrachaProtocolMessage{
			Stage:         messages.BrachaProtocolMessage_READY,
			SourceMessage: sourceMessage,
		})
	msgState.stage = SentReady
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
	messagesReceived :=
		p.messagesLog[sourceMessage.Author][sourceMessage.SeqNumber].receivedMessagesCnt

	delete(p.messagesLog[sourceMessage.Author], sourceMessage.SeqNumber)
	p.logger.OnDeliver(sourceMessage, messagesReceived)
}

func (p *Process) Receive(context actor.Context) {
	message := context.Message()
	switch message.(type) {
	case *messages.Simulate:
		msg := message.(*messages.Simulate)

		p.transactionManager = &manager.TransactionManager{
			TransactionsToSendOut: msg.Transactions,
		}
		p.transactionManager.SendOutTransaction(context, p)
	case *messages.BrachaProtocolMessage:
		msg := message.(*messages.BrachaProtocolMessage)
		sourceMessage := msg.SourceMessage
		value := protocols.ValueType(sourceMessage.Value)

		senderPid := utils.MakeCustomPid(context.Sender())
		p.logger.OnMessageReceived(senderPid, msg.Stamp)

		if p.delivered(sourceMessage) {
			return
		}

		msgState := p.initMessageState(sourceMessage)
		msgState.receivedMessagesCnt++

		switch msg.Stage {
		case messages.BrachaProtocolMessage_INITIAL:
			if msgState.stage == Init {
				p.broadcastEcho(context, sourceMessage, msgState)
			}
		case messages.BrachaProtocolMessage_ECHO:
			if msgState.stage == SentReady || msgState.receivedEcho[senderPid] {
				return
			}
			msgState.receivedEcho[senderPid] = true
			msgState.echoCount[value]++

			if msgState.echoCount[value] >= p.messagesForEcho {
				if msgState.stage == Init {
					p.broadcastEcho(context, sourceMessage, msgState)
				}
				if msgState.stage == SentEcho {
					p.broadcastReady(context, sourceMessage, msgState)
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
					p.broadcastEcho(context, sourceMessage, msgState)
				}
				if msgState.stage == SentEcho {
					p.broadcastReady(context, sourceMessage, msgState)
				}
			}
			if msgState.readyCount[value] >= p.messagesForDelivery {
				p.deliver(sourceMessage)
			}
		}
	}
}

func (p *Process) Broadcast(context actor.SenderContext, value int64) {
	message := &messages.BrachaProtocolMessage{
		Stage: messages.BrachaProtocolMessage_INITIAL,
		SourceMessage: &messages.SourceMessage{
			Author:    p.pid,
			SeqNumber: p.transactionCounter,
			Value:     value,
		},
	}
	p.broadcast(context, message)

	p.logger.OnTransactionInit(p.transactionCounter)

	p.transactionCounter++
}
