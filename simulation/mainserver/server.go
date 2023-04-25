package main

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
)

type MainServer struct {
	pids                  []*actor.PID
	startedProcessesCount int

	logger *log.Logger
}

func (ms *MainServer) InitMainServer(pids []*actor.PID, logger *log.Logger) {
	ms.pids = pids
	ms.startedProcessesCount = 0
	ms.logger = logger
}

func (ms *MainServer) simulate(context actor.SenderContext) {
	for _, pid := range ms.pids {
		context.Send(pid, &messages.Simulate{})
	}
}

func (ms *MainServer) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *messages.Started:
		ms.startedProcessesCount++
		if ms.startedProcessesCount == len(ms.pids) {
			ms.logger.Printf("Starting broadcast, timestamp: %d\n", utils.GetNow())
			ms.simulate(context)
		}
	}
}
