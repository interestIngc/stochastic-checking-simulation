package context

import (
	"log"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
	"sync"
	"time"
)

type ReliableContext struct {
	ProcessIndex int64
	Logger       *eventlogger.EventLogger

	retransmissionTimeoutNs int

	messageCounter int64

	writeChanMap map[int64]chan []byte

	receivedAcks map[int64]chan bool
	mutex       *sync.RWMutex
}

func (c *ReliableContext) InitContext(
	processIndex int64,
	logger *log.Logger,
	writeChanMap map[int64]chan []byte,
	retransmissionTimeoutNs int,
) {
	c.ProcessIndex = processIndex
	c.Logger = eventlogger.InitEventLogger(processIndex, logger)

	c.retransmissionTimeoutNs = retransmissionTimeoutNs
	c.messageCounter = 0

	c.writeChanMap = writeChanMap

	c.receivedAcks = make(map[int64]chan bool)
	c.mutex = &sync.RWMutex{}
}

func (c *ReliableContext) MakeNewMessage() *messages.Message {
	msg := &messages.Message{
		Sender: c.ProcessIndex,
		Stamp:  c.messageCounter,
	}
	c.messageCounter++
	return msg
}

func (c *ReliableContext) send(to int64, data []byte, stamp int64) {
	c.writeChanMap[to] <- data
	c.Logger.OnMessageSent(stamp)
}

func (c *ReliableContext) Send(to int64, msg *messages.Message) {
	data, e := utils.Marshal(msg)
	if e != nil {
		return
	}
	stamp := msg.Stamp

	c.send(to, data, stamp)
	ackChan := make(chan bool)

	c.mutex.Lock()
	c.receivedAcks[stamp] = ackChan
	c.mutex.Unlock()

	go func() {
		for {
			select {
			case <-ackChan:
				return
			case <-time.After(time.Duration(c.retransmissionTimeoutNs)):
				c.send(to, data, stamp)
			}
		}
	}()
}

func (c *ReliableContext) SendAck(sender int64, stamp int64) {
	msg := c.MakeNewMessage()
	msg.Content = &messages.Message_Ack{
		Ack: &messages.Ack{
			Sender: sender,
			Stamp:  stamp,
		},
	}

	data, e := utils.Marshal(msg)
	if e != nil {
		return
	}

	c.send(sender, data, msg.Stamp)
}

func (c *ReliableContext) OnAck(ack *messages.Ack) {
	c.receivedAcks[ack.Stamp] <- true
	c.Logger.OnAckReceived(ack.Stamp)
}
