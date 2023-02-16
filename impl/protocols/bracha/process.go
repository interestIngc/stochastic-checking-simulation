package bracha

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"math"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/messages"
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
	echoCount     map[protocols.ValueType]int
	readyCount    map[protocols.ValueType]int
	stage         Stage
}

func newMessageState() *messageState {
	ms := new(messageState)
	ms.receivedEcho = make(map[string]bool)
	ms.receivedReady = make(map[string]bool)
	ms.echoCount = make(map[protocols.ValueType]int)
	ms.readyCount = make(map[protocols.ValueType]int)
	ms.stage = Init
	return ms
}

type Process struct {
	actorPid   *actor.PID
	pid        string
	actorPids  map[string]*actor.PID
	msgCounter int64

	acceptedMessages map[string]map[int64]protocols.ValueType
	messagesLog      map[string]map[int64]*messageState

	messagesForEcho   int
	messagesForReady  int
	messagesForAccept int
}

func (p *Process) InitProcess(actorPid *actor.PID, actorPids []*actor.PID, parameters *config.Parameters) {
	p.actorPid = actorPid
	p.pid = utils.MakeCustomPid(actorPid)
	p.actorPids = make(map[string]*actor.PID)

	p.msgCounter = 0

	p.messagesForEcho = int(math.Ceil(float64(len(actorPids)+parameters.FaultyProcesses+1) / float64(2)))
	p.messagesForReady = parameters.FaultyProcesses + 1
	p.messagesForAccept = 2*parameters.FaultyProcesses + 1

	p.acceptedMessages = make(map[string]map[int64]protocols.ValueType)
	p.messagesLog = make(map[string]map[int64]*messageState)
	for _, currActorPid := range actorPids {
		pid := utils.MakeCustomPid(currActorPid)
		p.actorPids[pid] = currActorPid
		p.acceptedMessages[pid] = make(map[int64]protocols.ValueType)
		p.messagesLog[pid] = make(map[int64]*messageState)
	}
}

func (p *Process) initMessageState(msgData *messages.MessageData) *messageState {
	msgState := p.messagesLog[msgData.Author][msgData.SeqNumber]
	if msgState == nil {
		msgState = newMessageState()
		p.messagesLog[msgData.Author][msgData.SeqNumber] = msgState
	}
	return msgState
}

func (p *Process) broadcast(context actor.SenderContext, message *messages.BrachaMessage) {
	for _, pid := range p.actorPids {
		context.RequestWithCustomSender(pid, message, p.actorPid)
	}
}

func (p *Process) broadcastEcho(
	context actor.SenderContext,
	msgData *messages.MessageData,
	msgState *messageState) {
	p.broadcast(
		context,
		&messages.BrachaMessage{
			Stage:       messages.BrachaMessage_ECHO,
			MessageData: msgData,
		})
	msgState.stage = SentEcho
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	msgData *messages.MessageData,
	msgState *messageState) {
	p.broadcast(
		context,
		&messages.BrachaMessage{
			Stage:       messages.BrachaMessage_READY,
			MessageData: msgData,
		})
	msgState.stage = SentReady
}

func (p *Process) deliver(msgData *messages.MessageData) {
	p.acceptedMessages[msgData.Author][msgData.SeqNumber] = protocols.ValueType(msgData.Value)
	delete(p.messagesLog[msgData.Author], msgData.SeqNumber)

	fmt.Printf(
		"%s: Accepted transaction with seq number %d and value %d from %s in bracha broadcast\n",
		p.pid, msgData.SeqNumber, msgData.Value, msgData.Author)
}

func (p *Process) Receive(context actor.Context) {
	message := context.Message()
	switch message.(type) {
	case *messages.Broadcast:
		msg := message.(*messages.Broadcast)
		p.Broadcast(context, msg.Value)
	case *messages.BrachaMessage:
		msg := message.(*messages.BrachaMessage)
		msgData := msg.GetMessageData()
		value := protocols.ValueType(msgData.Value)

		senderPid := utils.MakeCustomPid(context.Sender())

		acceptedValue, accepted := p.acceptedMessages[msgData.Author][msgData.SeqNumber]
		if accepted {
			if acceptedValue != value {
				fmt.Printf("%s: Detected a duplicated seq number attack\n", p.pid)
			}
			return
		}

		msgState := p.initMessageState(msgData)

		switch msg.Stage {
		case messages.BrachaMessage_INITIAL:
			if msgState.stage == Init {
				p.broadcastEcho(context, msgData, msgState)
			}
		case messages.BrachaMessage_ECHO:
			if msgState.stage == SentReady || msgState.receivedEcho[senderPid] {
				return
			}
			msgState.receivedEcho[senderPid] = true
			msgState.echoCount[value]++

			if msgState.echoCount[value] >= p.messagesForEcho {
				if msgState.stage == Init {
					p.broadcastEcho(context, msgData, msgState)
				}
				if msgState.stage == SentEcho {
					p.broadcastReady(context, msgData, msgState)
				}
			}
		case messages.BrachaMessage_READY:
			if msgState.receivedReady[senderPid] {
				return
			}
			msgState.receivedReady[senderPid] = true
			msgState.readyCount[value]++

			if msgState.readyCount[value] >= p.messagesForReady {
				if msgState.stage == Init {
					p.broadcastEcho(context, msgData, msgState)
				}
				if msgState.stage == SentEcho {
					p.broadcastReady(context, msgData, msgState)
				}
			}
			if msgState.readyCount[value] >= p.messagesForAccept {
				p.deliver(msgData)
			}
		}
	}
}

func (p *Process) Broadcast(context actor.SenderContext, value int64) {
	message := &messages.BrachaMessage{
		Stage: messages.BrachaMessage_INITIAL,
		MessageData: &messages.MessageData{
			Author:    p.pid,
			SeqNumber: p.msgCounter,
			Value:     value,
		},
	}
	p.broadcast(context, message)

	p.msgCounter++
}
