package actor

import (
	"log"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
)

type ActorImpl interface {
	Start(
		context *context.ReliableContext,
		eventLogger *eventlogger.EventLogger,
	)
	ProcessMessage(message *messages.Message)
}

type Actor struct {
	context     *context.ReliableContext
	eventLogger *eventlogger.EventLogger

	actorImpl ActorImpl

	receivedMessages map[int32]map[int32]bool

	mailbox  *Mailbox
	readChan chan []byte
}

func (a *Actor) InitActor(
	processIndex int32,
	nodeAddresses []string,
	mainServerAddr string,
	actorImpl ActorImpl,
	logger *log.Logger,
	retransmissionTimeoutNs int,
) {
	nodeAddresses = append(nodeAddresses, mainServerAddr)
	a.receivedMessages = make(map[int32]map[int32]bool)
	for i := range nodeAddresses {
		a.receivedMessages[int32(i)] = make(map[int32]bool)
	}

	a.readChan = make(chan []byte, 500)
	writeChan := make(chan context.Packet)

	a.mailbox = newMailbox(processIndex, nodeAddresses, writeChan, a.readChan)

	a.actorImpl = actorImpl
	a.eventLogger = eventlogger.InitEventLogger(processIndex, logger)

	a.context =
		context.NewReliableContext(
			processIndex,
			writeChan,
			retransmissionTimeoutNs,
			a.eventLogger,
		)

	a.actorImpl.Start(a.context, a.eventLogger)

	a.receiveMessages()
}

func (a *Actor) receiveMessages() {
	for data := range a.readChan {
		msg, err := utils.Unmarshal(data)
		if err != nil {
			continue
		}

		content := msg.Content
		ack, isAck := content.(*messages.Message_Ack)
		if isAck {
			a.context.OnAck(ack.Ack)
			continue
		}

		sender := msg.Sender
		stamp := msg.Stamp

		a.eventLogger.OnMessageReceived(sender, stamp)

		a.context.SendAck(sender, stamp)

		if a.receivedMessages[sender][stamp] {
			continue
		}
		a.receivedMessages[sender][stamp] = true

		a.actorImpl.ProcessMessage(msg)
	}
}
