package context

import (
	"log"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
	"stochastic-checking-simulation/mailbox"
	"sync"
	"time"
)

type ReliableContext struct {
	ProcessIndex int32
	Logger       *eventlogger.EventLogger

	retransmissionTimeoutNs int

	messageCounter int32
	counterMutex   *sync.RWMutex

	writeChan chan mailbox.Packet

	receivedAcks map[int32]chan bool
	mutex        *sync.RWMutex
}

func (c *ReliableContext) InitContext(
	processIndex int32,
	logger *log.Logger,
	writeChan chan mailbox.Packet,
	retransmissionTimeoutNs int,
) {
	c.ProcessIndex = processIndex
	c.Logger = eventlogger.InitEventLogger(processIndex, logger)

	c.retransmissionTimeoutNs = retransmissionTimeoutNs
	c.messageCounter = 0

	c.writeChan = writeChan

	c.receivedAcks = make(map[int32]chan bool)
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

func (c *ReliableContext) send(to int32, msg *messages.Message) {
	data, e := utils.Marshal(msg)
	if e != nil {
		log.Println("Error when serializing message")
		return
	}
	c.writeChan <- mailbox.Packet{
		To:   to,
		Data: data,
	}
	c.Logger.OnMessageSent(msg.Stamp)
}

func (c *ReliableContext) Send(to int32, msg *messages.Message) {
	c.send(to, msg)

	c.mutex.Lock()
	ackChan := make(chan bool)
	c.receivedAcks[msg.Stamp] = ackChan
	c.mutex.Unlock()

	go func() {
		t := time.NewTicker(time.Duration(c.retransmissionTimeoutNs))
		for {
			select {
			case <-ackChan:
				return
			case <-t.C:
				msg.RetransmissionStamp++
				c.send(to, msg)
			}
		}
	}()
}

func (c *ReliableContext) SendAck(sender int32, stamp int32) {
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
	ackChan, pending := c.receivedAcks[ack.Stamp]
	c.mutex.RUnlock()

	if !pending {
		return
	}

	ackChan <- true
	c.mutex.Lock()
	delete(c.receivedAcks, ack.Stamp)
	c.mutex.Unlock()

	c.Logger.OnAckReceived(ack.Stamp)
}
