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
	counterMutex       *sync.RWMutex

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
	c.counterMutex = &sync.RWMutex{}
}

func (c *ReliableContext) MakeNewMessage() *messages.Message {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()

	msg := &messages.Message{
		Sender: c.ProcessIndex,
		Stamp:  c.messageCounter,
	}
	c.messageCounter++
	return msg
}

func (c *ReliableContext) send(to int64, msg *messages.Message) {
	data, e := utils.Marshal(msg)
	if e != nil {
		return
	}
	c.writeChanMap[to] <- data
	c.Logger.OnMessageSent(msg.Stamp)
}

func (c *ReliableContext) Send(to int64, msg *messages.Message) {
	c.send(to, msg)

	stamp := msg.Stamp
	ackChan := make(chan bool)

	c.mutex.Lock()
	c.receivedAcks[stamp] = ackChan
	c.mutex.Unlock()

	go func() {
		t := time.NewTicker(time.Duration(c.retransmissionTimeoutNs))
		select {
		case <- t.C:
			c.retransmit(to, msg)
		case <- ackChan:
			c.mutex.Lock()
			delete(c.receivedAcks, stamp)
			c.mutex.Unlock()
			return
		}
	}()
}

func (c *ReliableContext) retransmit(to int64, msg *messages.Message) {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()

	msg.Stamp = c.messageCounter
	c.messageCounter++
	c.send(to, msg)
}

func (c *ReliableContext) SendAck(sender int64, stamp int64) {
	msg := c.MakeNewMessage()
	msg.Content = &messages.Message_Ack{
		Ack: &messages.Ack{
			Sender: sender,
			Stamp:  stamp,
		},
	}

	c.send(sender, msg)
}

func (c *ReliableContext) OnAck(ack *messages.Ack) {
	c.mutex.RLock()
	ackChan, exists := c.receivedAcks[ack.Stamp]
	c.mutex.RUnlock()
	if !exists {
		return
	}

	ackChan <- true
	c.Logger.OnAckReceived(ack.Stamp)
}
