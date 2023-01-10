package broadcast

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"math"
	"stochastic-checking-simulation/messages"
	"stochastic-checking-simulation/utils"
)

type Stage int32
type ValueType int32

var MessagesForEcho = int(math.Ceil(float64(utils.ProcessCount + utils.FaultyProcesses + 1) / float64(2)))
const MessagesForReady = utils.FaultyProcesses + 1
const MessagesForAccept = 2 * utils.FaultyProcesses + 1

const (
	Init Stage = iota
	SentEcho
	SentReady
	Accepted
)

type MessageState struct {
	receivedEcho  map[*actor.PID]bool
	receivedReady map[*actor.PID]bool
	echoCount     map[ValueType]int
	readyCount    map[ValueType]int
	stage         Stage
}

func NewMessageState() *MessageState {
	ms := new(MessageState)
	ms.receivedEcho = make(map[*actor.PID]bool)
	ms.receivedReady = make(map[*actor.PID]bool)
	ms.echoCount = make(map[ValueType]int)
	ms.readyCount = make(map[ValueType]int)
	ms.stage = Init
	return ms
}

type Process struct {
	currPid          *actor.PID
	pids             []*actor.PID
	acceptedMessages map[*actor.PID]map[int32]ValueType
	messagesLog      map[*actor.PID]map[int32]*MessageState
}

func (p *Process) InitProcess(currPid *actor.PID, pids []*actor.PID) {
	p.currPid = currPid
	p.pids = pids
	p.acceptedMessages = make(map[*actor.PID]map[int32]ValueType)
	p.messagesLog = make(map[*actor.PID]map[int32]*MessageState)
	for _, pid := range pids {
		p.acceptedMessages[pid] = make(map[int32]ValueType)
		p.messagesLog[pid] = make(map[int32]*MessageState)
	}
}

func (p *Process) broadcast(context actor.SenderContext, message *messages.Message) {
	for _, pid := range p.pids {
		if pid != p.currPid {
			context.RequestWithCustomSender(pid, message, p.currPid)
		}
	}
}

func (p *Process) broadcastEcho(
	context actor.SenderContext,
	initialMessage *messages.Message,
	msgState *MessageState) {
	p.broadcast(
		context,
		&messages.Message{
			Stage:     messages.Message_ECHO,
			Value:     initialMessage.Value,
			SeqNumber: initialMessage.SeqNumber,
			Author:    initialMessage.Author,
		})
	msgState.stage = SentEcho
	msgState.echoCount[ValueType(initialMessage.Value)]++
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	initialMessage *messages.Message,
	msgState *MessageState) {
	p.broadcast(
		context,
		&messages.Message{
			Stage:     messages.Message_READY,
			Value:     initialMessage.Value,
			SeqNumber: initialMessage.SeqNumber,
			Author:    initialMessage.Author,
		})
	msgState.stage = SentReady
	msgState.readyCount[ValueType(initialMessage.Value)]++
	p.checkForAccept(initialMessage, msgState)
}

func (p *Process) checkForAccept(msg *messages.Message, msgState *MessageState) {
	if msgState.readyCount[ValueType(msg.Value)] >= MessagesForAccept {
		msgState.stage = Accepted
		p.acceptedMessages[msg.Author][msg.SeqNumber] = ValueType(msg.Value)
		fmt.Printf(
			"%s accepted value %d with seq number %d\n",
			p.currPid.String(), msg.Value, msg.SeqNumber)
	}
}

func (p *Process) initMessageState(msg *messages.Message) {
	if p.messagesLog[msg.Author][msg.SeqNumber] == nil {
		p.messagesLog[msg.Author][msg.SeqNumber] = NewMessageState()
	}
}

func (p *Process) Receive(context actor.Context) {
	msg, ok := context.Message().(*messages.Message)
	if !ok {
		return
	}
	p.initMessageState(msg)

	msgState := p.messagesLog[msg.Author][msg.SeqNumber]
	senderId := context.Sender()
	value := ValueType(msg.Value)

	switch msg.Stage {
	case messages.Message_INITIAL:
		if msgState.stage == Init {
			p.broadcastEcho(context, msg, msgState)
		}
	case messages.Message_ECHO:
		if msgState.stage == SentReady || msgState.receivedEcho[senderId] {
			return
		}
		msgState.receivedEcho[senderId] = true
		msgState.echoCount[value]++

		if msgState.echoCount[value] >= MessagesForEcho {
			if msgState.stage == Init {
				p.broadcastEcho(context, msg, msgState)
			}
			if msgState.stage == SentEcho {
				p.broadcastReady(context, msg, msgState)
			}
		}
	case messages.Message_READY:
		if msgState.stage == Accepted || msgState.receivedReady[senderId] {
			return
		}
		msgState.receivedReady[senderId] = true
		msgState.readyCount[value]++

		if msgState.readyCount[value] >= MessagesForReady {
			if msgState.stage == Init {
				p.broadcastEcho(context, msg, msgState)
			}
			if msgState.stage == SentEcho {
				p.broadcastReady(context, msg, msgState)
			}
		}
		p.checkForAccept(msg, msgState)
	}
}

func (p *Process) Broadcast(context actor.SenderContext, value int32, seqNumber int32) {
	message := &messages.Message{
		Stage:     messages.Message_INITIAL,
		Author:    p.currPid,
		SeqNumber: seqNumber,
		Value:     value,
	}
	p.broadcast(context, message)

	msgState := NewMessageState()
	p.messagesLog[p.currPid][seqNumber] = msgState
	p.broadcastEcho(context, message, msgState)
}
