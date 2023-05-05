package context

import (
	"google.golang.org/protobuf/proto"
	"log"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
)

type ReliableContext struct {
	ProcessIndex int64
	Logger       *eventlogger.EventLogger

	retransmissionTimeoutNs int

	messageCounter          int64

	writeChanMap map[int64]chan []byte

	//receivedAck      map[int64]bool
	//mutex sync.RWMutex
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

	//c.receivedAck = make(map[int64]bool)
	//c.mutex = sync.RWMutex{}
}

func (c *ReliableContext) MakeNewMessage() *messages.Message {
	msg := &messages.Message{
		Sender: c.ProcessIndex,
		Stamp:  c.messageCounter,
	}
	c.messageCounter++
	return msg
}

func (c *ReliableContext) send(to int64, msg *messages.Message) {
	data, e := proto.Marshal(msg)
	if e != nil {
		log.Printf("Could not marshal message, %e", e)
		return
	}
	c.writeChanMap[to] <- data

	c.Logger.OnMessageSent(msg.Stamp)
}

func (c *ReliableContext) Send(to int64, msg *messages.Message) {
	c.send(to, msg)
	//go func() {
	//	for {
	//		time.Sleep(time.Duration(c.retransmissionTimeoutNs))
	//
	//		c.mutex.RLock()
	//		received := c.receivedAck[msg.Stamp]
	//		c.mutex.RUnlock()
	//
	//		if received {
	//			return
	//		}
	//
	//		c.send(to, msg)
	//	}
	//}()
}

//func (c *ReliableContext) SendAck(sender int64, stamp int64) {
//	msg := c.MakeNewMessage()
//	msg.Content = &messages.Message_Ack{
//		Ack: &messages.Ack{
//			Sender: sender,
//			Stamp:  stamp,
//		},
//	}
//	c.send(sender, msg)
//}

//func (c *ReliableContext) OnAck(ack *messages.Ack) {
//	c.mutex.Lock()
//	defer c.mutex.Unlock()
//
//	stamp := ack.Stamp
//	c.receivedAck[stamp] = true
//	c.Logger.OnAckReceived(stamp)
//}
