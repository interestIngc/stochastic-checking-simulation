package context

import (
	"log"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
	"sync"
)

type Packet struct {
	To   int32
	Data []byte
}

// ReliableContext allows a process to send messages reliably, with possible retransmissions.
// It retransmits message with a predefined timeout until acknowledgement is received.
type ReliableContext struct {
	processIndex int32
	eventLogger  *eventlogger.EventLogger

	retransmissionTimeoutNs int

	messageCounter int32
	counterMutex   *sync.RWMutex

	writeChan chan Packet

	receivedAcks map[int32]chan bool
	mutex        *sync.RWMutex
}

func NewReliableContext(
	processIndex int32,
	writeChan chan Packet,
	retransmissionTimeoutNs int,
	eventLogger *eventlogger.EventLogger,
) *ReliableContext {
	c := new(ReliableContext)
	c.processIndex = processIndex

	c.retransmissionTimeoutNs = retransmissionTimeoutNs
	c.eventLogger = eventLogger

	c.writeChan = writeChan

	c.messageCounter = 0

	c.receivedAcks = make(map[int32]chan bool)
	c.mutex = &sync.RWMutex{}
	c.counterMutex = &sync.RWMutex{}

	return c
}

func (c *ReliableContext) MakeNewMessage() *messages.Message {
	c.counterMutex.Lock()
	defer c.counterMutex.Unlock()
	msg := &messages.Message{
		Sender: c.processIndex,
		Stamp:  c.messageCounter,
	}
	c.messageCounter++
	return msg
}

func (c *ReliableContext) send(to int32, msg *messages.Message) {
	data, e := utils.Marshal(msg)
	if e != nil {
		log.Printf("Error while serializing message happened: %e\n", e)
		return
	}
	c.writeChan <- Packet{
		To:   to,
		Data: data,
	}
	c.eventLogger.OnMessageSent(msg.Stamp)
}

func (c *ReliableContext) Send(to int32, msg *messages.Message) {
	//c.mutex.Lock()
	//ackChan := make(chan bool)
	//c.receivedAcks[msg.Stamp] = ackChan
	//c.mutex.Unlock()
	//
	c.send(to, msg)

	//go func() {
	//	t := time.NewTicker(time.Duration(c.retransmissionTimeoutNs))
	//	for {
	//		select {
	//		case <-ackChan:
	//			return
	//		case <-t.C:
	//			msg.RetransmissionStamp++
	//			c.send(to, msg)
	//		}
	//	}
	//}()
}

//func (c *ReliableContext) SendAck(sender int32, stamp int32) {
//	msg := c.MakeNewMessage()
//	msg.Content = &messages.Message_Ack{
//		Ack: &messages.Ack{
//			Sender: sender,
//			Stamp:  stamp,
//		},
//	}
//
//	c.send(sender, msg)
//}

//func (c *ReliableContext) OnAck(ack *messages.Ack) {
//	ackChan, pending := c.receivedAcks[ack.Stamp]
//	if !pending {
//		return
//	}
//
//	ackChan <- true
//	delete(c.receivedAcks, ack.Stamp)
//
//	c.eventLogger.OnAckReceived(ack.Stamp)
//}
