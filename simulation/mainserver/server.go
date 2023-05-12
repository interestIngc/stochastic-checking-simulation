package main

import (
	"log"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
	"stochastic-checking-simulation/mailbox"
)

type MainServer struct {
	n int

	logger          *log.Logger
	reliableContext *context.ReliableContext

	receivedMessages map[int32]bool

	mailbox  *mailbox.Mailbox
	readChan chan []byte
}

func (ms *MainServer) InitMainServer(
	actorPids []string,
	logger *log.Logger,
	retransmissionTimeoutNs int,
) {
	ms.n = len(actorPids) - 1
	ms.logger = logger

	ms.readChan = make(chan []byte, 500)

	ms.receivedMessages = make(map[int32]bool)

	writeChan := make(chan mailbox.Packet)

	id := int32(ms.n)
	ms.mailbox = mailbox.NewMailbox(id, actorPids, writeChan, ms.readChan)
	ms.mailbox.SetUp()

	ms.reliableContext = &context.ReliableContext{}
	ms.reliableContext.InitContext(id, logger, writeChan, retransmissionTimeoutNs)

	ms.receiveMessages()
}

func (ms *MainServer) simulate() {
	for pid := 0; pid < ms.n; pid++ {
		msg := ms.reliableContext.MakeNewMessage()
		msg.Content = &messages.Message_Simulate{
			Simulate: &messages.Simulate{},
		}
		ms.reliableContext.Send(int32(pid), msg)
	}
}

func (ms *MainServer) receiveMessages() {
	for data := range ms.readChan {
		msg, err := utils.Unmarshal(data)
		if err != nil {
			continue
		}

		switch content := msg.Content.(type) {
		case *messages.Message_Ack:
			ms.reliableContext.OnAck(content.Ack)
		case *messages.Message_Started:
			sender := msg.Sender
			stamp := msg.Stamp

			ms.reliableContext.Logger.OnMessageReceived(sender, stamp)

			ms.reliableContext.SendAck(sender, stamp)

			if ms.receivedMessages[sender] {
				continue
			}
			ms.receivedMessages[sender] = true

			if len(ms.receivedMessages) == ms.n {
				ms.logger.Printf("Starting broadcast, timestamp: %d\n", utils.GetNow())
				ms.simulate()
			}
		}
	}
}
