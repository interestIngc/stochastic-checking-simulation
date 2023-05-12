package main

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
	processIndex int32

	context *context.ReliableContext

	receivedMessages map[int32]map[int32]bool

	transactionsToSendOut    int
	transactionInitTimeoutNs int

	process  protocols.Process
	mailbox  *mailbox.Mailbox
	readChan chan []byte

	ownDeliveredTransactions chan bool
	stressTest               bool
}

func (a *Actor) InitActor(
	processIndex int32,
	actorPids []string,
	parameters *parameters.Parameters,
	logger *log.Logger,
	mainServerAddr string,
	transactionsToSendOut int,
	transactionInitTimeoutNs int,
	process protocols.Process,
	retransmissionTimeoutNs int,
	stressTest bool,
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

	a.receivedMessages = make(map[int32]map[int32]bool)
	for i := 0; i <= n; i++ {
		a.receivedMessages[int32(i)] = make(map[int32]bool)
	}

	a.readChan = make(chan []byte, 500)
	writeChan := make(chan mailbox.Packet)

	a.mailbox = mailbox.NewMailbox(processIndex, pids, writeChan, a.readChan)
	a.mailbox.SetUp()

	a.context = &context.ReliableContext{}
	a.context.InitContext(processIndex, logger, writeChan, retransmissionTimeoutNs)

	a.ownDeliveredTransactions = make(chan bool, 200)

	a.process = process
	a.process.InitProcess(
		processIndex,
		actorPids,
		parameters,
		a.context.Logger,
		a.ownDeliveredTransactions,
		stressTest,
	)

	startedMessage := a.context.MakeNewMessage()
	startedMessage.Content = &messages.Message_Started{
		Started: &messages.Started{},
	}
	a.context.Send(int32(n), startedMessage)

	a.stressTest = stressTest

	a.receiveMessages()
}

func (a *Actor) receiveMessages() {
	for data := range a.readChan {
		msg := &messages.Message{}

		err := utils.Unmarshal(data, msg)
		if err != nil {
			continue
		}

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
			continue
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
	if a.stressTest {
		a.sendMsg()
		for {
			select {
			case <-a.ownDeliveredTransactions:
				a.sendMsg()
			}
		}
	} else {
		for i := 0; i < a.transactionsToSendOut; i++ {
			a.sendMsg()
			time.Sleep(time.Duration(a.transactionInitTimeoutNs))
		}
	}
}

func (a *Actor) sendMsg() {
	msg := a.context.MakeNewMessage()
	msg.Content = &messages.Message_Broadcast{
		Broadcast: &messages.Broadcast{
			Value: int32(rand.Int() % 1000000),
		},
	}
	a.context.Send(a.processIndex, msg)
}
