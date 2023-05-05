package handler

import (
	"log"
	"math/rand"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/utils"
	"stochastic-checking-simulation/mailbox"
	"time"
)

type Actor struct {
	processIndex int64

	context *context.ReliableContext

	receivedMessages map[int64]map[int64]bool

	transactionsToSendOut    int
	transactionInitTimeoutNs int

	process protocols.Process
	mailbox *mailbox.Mailbox
	readChan  chan []byte
}

func (a *Actor) InitActor(
	processIndex int64,
	actorPids []string,
	parameters *parameters.Parameters,
	logger *log.Logger,
	mainServerAddr string,
	transactionsToSendOut int,
	transactionInitTimeoutNs int,
	process protocols.Process,
) {
	n := len(actorPids)

	a.processIndex = processIndex

	pids := make([]string, n+1)
	for i := 0; i < n; i++ {
		pids[i] = actorPids[i]
	}
	pids[n] = mainServerAddr

	a.transactionsToSendOut = transactionsToSendOut
	a.transactionInitTimeoutNs = transactionInitTimeoutNs

	a.receivedMessages = make(map[int64]map[int64]bool)
	writeChanMap := make(map[int64]chan []byte)
	for i := 0; i <= n; i++ {
		writeChanMap[int64(i)] = make(chan []byte, 500)
		a.receivedMessages[int64(i)] = make(map[int64]bool)
	}

	a.readChan = make(chan []byte, 500)

	a.mailbox = mailbox.NewMailbox(processIndex, pids, writeChanMap, a.readChan)
	a.mailbox.SetUp()

	a.context = &context.ReliableContext{}
	a.context.InitContext(processIndex, logger, writeChanMap, parameters.RetransmissionTimeoutNs)

	a.process = process
	a.process.InitProcess(processIndex, actorPids, parameters, a.context.Logger)

	startedMessage := a.context.MakeNewMessage()
	startedMessage.Content = &messages.Message_Started{
		Started: &messages.Started{},
	}
	a.context.Send(int64(n), startedMessage)

	a.receiveMessages()
}

func (a *Actor) receiveMessages() {
	for data := range a.readChan {
		msg := &messages.Message{}

		err := utils.Unmarshal(data, msg)
		if err != nil {
			continue
		}

		//log.Printf("Received message: %s\n", msg.String())

		content := msg.Content
		ack, ok := content.(*messages.Message_Ack)
		if ok {
			a.context.OnAck(ack.Ack)
			continue
		}

		sender := msg.Sender
		stamp := msg.Stamp

		a.context.Logger.OnMessageReceived(sender, stamp)

		a.context.SendAck(sender, stamp)

		if a.receivedMessages[sender][stamp] {
			return
		}
		a.receivedMessages[sender][stamp] = true

		switch c := content.(type) {
		case *messages.Message_Broadcast:
			a.process.Broadcast(a.context, c.Broadcast.Value)
		case *messages.Message_Simulate:
			a.context.Logger.OnSimulationStart()
			go a.Simulate()
		case *messages.Message_BroadcastInstanceMessage:
			a.process.HandleMessage(a.context, sender, c.BroadcastInstanceMessage)
		}
	}
}

func (a *Actor) Simulate() {
	for i := 0; i < a.transactionsToSendOut; i++ {
		msg := a.context.MakeNewMessage()
		msg.Content = &messages.Message_Broadcast{
			Broadcast: &messages.Broadcast{
				Value: int64(rand.Int()),
			},
		}
		a.context.Send(a.processIndex, msg)
		time.Sleep(time.Duration(a.transactionInitTimeoutNs))
	}
}
