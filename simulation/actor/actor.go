package actor

import (
	"log"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
)

const ChannelSize = 1024

// ActorInstance interface represents an instance of actor: either mainserver or node.
// It exports two methods:
// Start sets up current instance of actor;
// ProcessMessage processes the given message.
type ActorInstance interface {
	Start(
		context *context.ReliableContext,
		eventLogger *eventlogger.EventLogger,
	)
	ProcessMessage(message *messages.Message)
}

// Actor represents a basic actor.
// It reads incoming messages, processes them and potentially sends messages to others.
type Actor struct {
	context     *context.ReliableContext
	eventLogger *eventlogger.EventLogger

	actorInstance ActorInstance

	receivedMessages map[int32]map[int32]bool

	mailbox  *Mailbox
	readChan chan []byte
}

func (a *Actor) InitActor(
	processIndex int32,
	nodeAddresses []string,
	actorInstance ActorInstance,
	logger *log.Logger,
	retransmissionTimeoutNs int,
	messageDelay int,
) {
	a.receivedMessages = make(map[int32]map[int32]bool)
	for i := range nodeAddresses {
		a.receivedMessages[int32(i)] = make(map[int32]bool)
	}

	a.readChan = make(chan []byte, ChannelSize)
	writeChan := make(chan context.Packet, ChannelSize)

	a.mailbox = newMailbox(processIndex, nodeAddresses, writeChan, a.readChan, messageDelay)

	a.actorInstance = actorInstance
	a.eventLogger = eventlogger.InitEventLogger(processIndex, logger)

	a.context =
		context.NewReliableContext(
			processIndex,
			writeChan,
			retransmissionTimeoutNs,
			a.eventLogger,
		)

	a.actorInstance.Start(a.context, a.eventLogger)

	a.receiveMessages()
}

func (a *Actor) receiveMessages() {
	for data := range a.readChan {
		msg, err := utils.Unmarshal(data)
		if err != nil {
			continue
		}

		//content := msg.Content
		//ack, isAck := content.(*messages.Message_Ack)
		//if isAck {
		//	a.context.OnAck(ack.Ack)
		//	continue
		//}

		sender := msg.Sender
		stamp := msg.Stamp

		a.eventLogger.OnMessageReceived(sender, stamp)

		//a.context.SendAck(sender, stamp)

		if a.receivedMessages[sender][stamp] {
			continue
		}
		a.receivedMessages[sender][stamp] = true

		a.actorInstance.ProcessMessage(msg)
	}
}
