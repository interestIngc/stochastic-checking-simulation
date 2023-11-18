package main

import (
	"math/rand"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"time"
)

// Node represents an actor executing the reliable broadcast protocol.
type Node struct {
	processIndex int32
	pids         []string

	parameters               *parameters.Parameters
	transactionsToSendOut    int
	transactionInitTimeoutNs int

	process                  protocols.Process
	ownDeliveredTransactions chan bool
	stressTest               bool

	context     *context.ReliableContext
	eventLogger *eventlogger.EventLogger
}

func (node *Node) Start(
	context *context.ReliableContext,
	eventLogger *eventlogger.EventLogger,
) {
	node.context = context
	node.eventLogger = eventLogger

	node.ownDeliveredTransactions = make(chan bool, 200)

	mainServerAddr := int32(len(node.pids)) - 1
	node.process.InitProcess(
		node.processIndex,
		node.pids[:mainServerAddr],
		node.parameters,
		node.context,
		node.eventLogger,
		node.ownDeliveredTransactions,
		node.stressTest,
	)

	startedMessage := node.context.MakeNewMessage()
	startedMessage.Content = &messages.Message_Started{
		Started: &messages.Started{},
	}
	node.context.Send(mainServerAddr, startedMessage)
}

func (node *Node) ProcessMessage(message *messages.Message) {
	switch c := message.Content.(type) {
	case *messages.Message_Broadcast:
		node.process.Broadcast(c.Broadcast.Value)
	case *messages.Message_Simulate:
		node.eventLogger.OnSimulationStart()
		go node.simulate()
	case *messages.Message_BroadcastInstanceMessage:
		node.process.HandleMessage(message.Sender, c.BroadcastInstanceMessage)
	}
}

func (node *Node) simulate() {
	if node.stressTest {
		node.doBroadcast()
		for {
			select {
			case <-node.ownDeliveredTransactions:
				node.doBroadcast()
			}
		}
	} else {
		for i := 0; i < node.transactionsToSendOut; i++ {
			node.doBroadcast()
			time.Sleep(time.Duration(node.transactionInitTimeoutNs))
		}
	}
}

func (node *Node) doBroadcast() {
	msg := node.context.MakeNewMessage()
	msg.Content = &messages.Message_Broadcast{
		Broadcast: &messages.Broadcast{
			Value: int32(rand.Int() % 1000000),
		},
	}
	node.context.Send(node.processIndex, msg)
}
