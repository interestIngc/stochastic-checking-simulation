package main

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
)

type MainServer struct {
	actorPids        []*actor.PID
	startedProcesses map[int64]bool

	logger          *log.Logger
	reliableContext *context.ReliableContext
}

func (ms *MainServer) InitMainServer(
	actorPids []*actor.PID,
	logger *log.Logger,
) {
	n := len(actorPids)
	ms.actorPids = actorPids
	ms.logger = logger

	ms.startedProcesses = make(map[int64]bool)

	ms.reliableContext = &context.ReliableContext{}
	ms.reliableContext.InitContext(int64(n), n+1, logger)
}

func (ms *MainServer) simulate(context actor.Context) {
	for _, pid := range ms.actorPids {
		msg := ms.reliableContext.MakeNewMessage()
		msg.Content = &messages.Message_Simulate{
			Simulate: &messages.Simulate{},
		}
		ms.reliableContext.Send(context, pid, msg)
	}
}

func (ms *MainServer) Receive(context actor.Context) {
	switch message := context.Message().(type) {
	case *messages.Ack:
		ms.reliableContext.OnAck(message.Stamp)
	case *messages.Message:
		sender := message.Sender
		stamp := message.Stamp

		ms.reliableContext.SendAck(context, ms.actorPids[sender], sender, stamp)

		if ms.reliableContext.ReceivedMessage(sender, stamp) {
			return
		}

		switch content := message.Content.(type) {
		case *messages.Message_Started:
			startedMsg := content.Started
			ms.logger.Printf("Received message: %s\n", startedMsg.ToString())
			ms.startedProcesses[sender] = true
			if len(ms.startedProcesses) == len(ms.actorPids) {
				ms.logger.Printf("Starting broadcast, timestamp: %d\n", utils.GetNow())
				ms.simulate(context)
			}
		}
	}
}
