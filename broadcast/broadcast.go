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

type MessageData struct {
	receivedEcho map[*actor.PID]bool
	receivedReady map[*actor.PID]bool
	echoCount map[ValueType]int
	readyCount map[ValueType]int
	stage Stage
}

func NewMessageData() *MessageData {
	md := new(MessageData)
	md.receivedEcho = make(map[*actor.PID]bool)
	md.receivedReady = make(map[*actor.PID]bool)
	md.echoCount = make(map[ValueType]int)
	md.readyCount = make(map[ValueType]int)
	md.stage = Init
	return md
}

type Process struct {
	currPid *actor.PID
	pids []*actor.PID
	messagesInfo map[*actor.PID]map[int32]*MessageData
}

func (p *Process) InitProcess(currPid *actor.PID, pids []*actor.PID) {
	p.currPid = currPid
	p.pids = pids
	p.messagesInfo = make(map[*actor.PID]map[int32]*MessageData)
	for _, pid := range pids {
		p.messagesInfo[pid] = make(map[int32]*MessageData)
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
	msgData *MessageData) {
	p.broadcast(
		context,
		&messages.Message{
			Stage: messages.Message_ECHO,
			Value: initialMessage.Value,
			SeqNumber: initialMessage.SeqNumber,
			Author: initialMessage.Author,
		})
	msgData.stage = SentEcho
	msgData.echoCount[ValueType(initialMessage.Value)]++
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	initialMessage *messages.Message,
	msgData *MessageData) {
	p.broadcast(
		context,
		&messages.Message{
			Stage:     messages.Message_READY,
			Value:     initialMessage.Value,
			SeqNumber: initialMessage.SeqNumber,
			Author:    initialMessage.Author,
		})
	msgData.stage = SentReady
	msgData.readyCount[ValueType(initialMessage.Value)]++
	p.checkForAccept(initialMessage, msgData)
}

func (p *Process) checkForAccept(msg *messages.Message, msgData *MessageData) {
	if msgData.readyCount[ValueType(msg.Value)] >= MessagesForAccept {
		msgData.stage = Accepted
		fmt.Printf(
			"%s accepted value %d with seq number %d\n",
			p.currPid.String(), msg.Value, msg.SeqNumber)
	}
}

func (p *Process) initMessageState(msg *messages.Message) {
	if p.messagesInfo[msg.Author][msg.SeqNumber] == nil {
		p.messagesInfo[msg.Author][msg.SeqNumber] = NewMessageData()
	}
}

func (p *Process) Receive(context actor.Context) {
	msg, ok := context.Message().(*messages.Message)
	if !ok {
		return
	}
	p.initMessageState(msg)

	msgData := p.messagesInfo[msg.Author][msg.SeqNumber]
	senderId := context.Sender()
	value := ValueType(msg.Value)
	//fmt.Printf("%s received message from %s\n", p.currPid.String(), senderId.String())

	switch msg.Stage {
	case messages.Message_INITIAL:
		if msgData.stage == Init {
			p.broadcastEcho(context, msg, msgData)
		}
	case messages.Message_ECHO:
		if msgData.stage == SentReady || msgData.receivedEcho[senderId] {
			return
		}
		msgData.receivedEcho[senderId] = true
		msgData.echoCount[value]++

		if msgData.echoCount[value] >= MessagesForEcho {
			if msgData.stage == Init {
				p.broadcastEcho(context, msg, msgData)
			}
			if msgData.stage == SentEcho {
				p.broadcastReady(context, msg, msgData)
			}
		}
	case messages.Message_READY:
		if msgData.stage == Accepted || msgData.receivedReady[senderId] {
			return
		}
		msgData.receivedReady[senderId] = true
		msgData.readyCount[value]++

		if msgData.readyCount[value] >= MessagesForReady {
			if msgData.stage == Init {
				p.broadcastEcho(context, msg, msgData)
			}
			if msgData.stage == SentEcho {
				p.broadcastReady(context, msg, msgData)
			}
		}
		p.checkForAccept(msg, msgData)
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

	msgData := NewMessageData()
	p.messagesInfo[p.currPid][seqNumber] = msgData
	p.broadcastEcho(context, message, msgData)
}
