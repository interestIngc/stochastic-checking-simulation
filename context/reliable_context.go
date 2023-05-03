package context

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"time"
)

type ReliableContext struct {
	ProcessIndex int64
	Logger       *eventlogger.EventLogger

	retransmissionTimeoutMs int64
	messageCounter          int64

	receivedMessages map[int64]map[int64]bool
	receivedAck      map[int64]bool
}

func (c *ReliableContext) InitContext(processIndex int64, n int, logger *log.Logger) {
	c.ProcessIndex = processIndex
	c.Logger = eventlogger.InitEventLogger(processIndex, logger)

	c.retransmissionTimeoutMs = 5000000000
	c.messageCounter = 0

	c.receivedMessages = make(map[int64]map[int64]bool)
	c.receivedAck = make(map[int64]bool)

	for i := 0; i < n; i++ {
		c.receivedMessages[int64(i)] = make(map[int64]bool)
	}
}

func (c *ReliableContext) MakeNewMessage() *messages.Message {
	msg := &messages.Message{
		Sender: c.ProcessIndex,
		Stamp:  c.messageCounter,
	}
	c.messageCounter++
	return msg
}

func (c *ReliableContext) Send(context actor.Context, to *actor.PID, msg *messages.Message) {
	context.Send(to, msg)
	c.Logger.OnMessageSent(msg.Stamp)

	context.ReenterAfter(
		actor.NewFuture(context.ActorSystem(), time.Duration(c.retransmissionTimeoutMs)),
		func(res interface{}, err error) {
			if !c.receivedAck[msg.Stamp] {
				c.Send(context, to, msg)
			}
		},
	)
}

func (c *ReliableContext) SendAck(context actor.Context, to *actor.PID, sender int64, stamp int64) {
	context.Send(
		to,
		&messages.Ack{
			Sender: sender,
			Stamp:  stamp,
		},
	)
}

func (c *ReliableContext) OnAck(stamp int64) {
	c.receivedAck[stamp] = true
	c.Logger.OnAckReceived(stamp)
}

func (c *ReliableContext) ReceivedMessage(sender int64, stamp int64) bool {
	if c.receivedMessages[sender][stamp] {
		return true
	}

	c.receivedMessages[sender][stamp] = true
	return false
}
