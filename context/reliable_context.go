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
	counterMutex       *sync.RWMutex

	writeChanMap chan mailbox.Destination

	receivedAcks map[int32]chan bool
	mutex       *sync.RWMutex
}

func (c *ReliableContext) InitContext(
	processIndex int32,
	logger *log.Logger,
	writeChanMap chan mailbox.Destination,
	retransmissionTimeoutNs int,
) {
	c.ProcessIndex = processIndex
	c.Logger = eventlogger.InitEventLogger(processIndex, logger)

	c.retransmissionTimeoutNs = retransmissionTimeoutNs
	c.messageCounter = 0

	c.writeChanMap = writeChanMap

	c.receivedAcks = make(map[int32]chan bool)
}

func (c *ReliableContext) MakeNewMessage() *messages.Message {
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
	c.writeChanMap <- mailbox.Destination{
		To: to,
		Data: data,
	}
	c.Logger.OnMessageSent(msg.Stamp)
}

func (c *ReliableContext) Send(to int32, msg *messages.Message) {
	c.send(to, msg)

	stamp := msg.Stamp
	ackChan := make(chan bool)

	c.receivedAcks[stamp] = ackChan

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
	ackChan, exists := c.receivedAcks[ack.Stamp]
	if !exists {
		return
	}
	ackChan <- true
	delete(c.receivedAcks, ack.Stamp)
	c.Logger.OnAckReceived(ack.Stamp)
}
