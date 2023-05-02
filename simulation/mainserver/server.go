package main

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
)

type MainServer struct {
	pids             []*actor.PID
	startedProcesses map[int64]bool

	logger *log.Logger
}

func (ms *MainServer) InitMainServer(pids []*actor.PID, logger *log.Logger) {
	ms.pids = pids
	ms.logger = logger
	ms.startedProcesses = make(map[int64]bool)
}

func (ms *MainServer) simulate(context actor.SenderContext) {
	for _, pid := range ms.pids {
		context.Send(pid, &messages.Simulate{})
	}
}

func (ms *MainServer) Receive(context actor.Context) {
	switch message := context.Message().(type) {
	case *messages.Started:
		ms.logger.Printf("Received message: %s\n", message.ToString())
		ms.startedProcesses[message.Sender] = true
		if len(ms.startedProcesses) == len(ms.pids) {
			ms.logger.Printf("Starting broadcast, timestamp: %d\n", utils.GetNow())
			ms.simulate(context)
		}
	}
}
