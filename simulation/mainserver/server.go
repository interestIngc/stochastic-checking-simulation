package main

import (
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
)

type MainServer struct {
	n int

	context     *context.ReliableContext
	eventLogger *eventlogger.EventLogger

	connectedNodes map[int32]bool
}

func (ms *MainServer) Start(
	context *context.ReliableContext,
	eventLogger *eventlogger.EventLogger,
) {
	ms.context = context
	ms.eventLogger = eventLogger
	ms.connectedNodes = make(map[int32]bool)
}

func (ms *MainServer) ProcessMessage(message *messages.Message) {
	ms.connectedNodes[message.Sender] = true

	if len(ms.connectedNodes) == ms.n {
		ms.eventLogger.OnBroadcastStart()
		ms.simulate()
	}
}

func (ms *MainServer) simulate() {
	for pid := 0; pid < ms.n; pid++ {
		msg := ms.context.MakeNewMessage()
		msg.Content = &messages.Message_Simulate{
			Simulate: &messages.Simulate{},
		}
		ms.context.Send(int32(pid), msg)
	}
}
