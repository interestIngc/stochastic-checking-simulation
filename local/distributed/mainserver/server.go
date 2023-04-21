package main

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
)

type MainServer struct {
	currPid          *actor.PID
	processCount     int
	startedProcesses *actor.PIDSet

	logger *log.Logger
}

func (ms *MainServer) InitMainServer(currPid *actor.PID, processCount int, logger *log.Logger) {
	ms.currPid = currPid
	ms.processCount = processCount
	ms.startedProcesses = actor.NewPIDSet()
	ms.logger = logger
}

func (ms *MainServer) simulate(context actor.SenderContext) {
	for _, pid := range ms.startedProcesses.Values() {
		context.RequestWithCustomSender(
			pid,
			&messages.Simulate{},
			ms.currPid)
	}
}

func (ms *MainServer) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *messages.Started:
		ms.startedProcesses.Add(context.Sender())
		if ms.startedProcesses.Len() == ms.processCount {
			ms.logger.Printf("Starting broadcast, timestamp: %d\n", utils.GetNow())
			ms.simulate(context)
		}
	}
}
