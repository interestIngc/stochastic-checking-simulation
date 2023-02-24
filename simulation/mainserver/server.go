package main

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"stochastic-checking-simulation/impl/messages"
)

type MainServer struct {
	currPid          *actor.PID
	processCount     int
	times            int
	startedProcesses *actor.PIDSet
}

func (ms *MainServer) InitMainServer(currPid *actor.PID, processCount int, times int) {
	ms.currPid = currPid
	ms.processCount = processCount
	ms.times = times
	ms.startedProcesses = actor.NewPIDSet()
}

func (ms *MainServer) simulate(context actor.SenderContext) {
	for _, pid := range ms.startedProcesses.Values() {
		context.RequestWithCustomSender(
			pid,
			&messages.Simulate{
				Transactions: int64(ms.times),
			},
			ms.currPid)
	}
}

func (ms *MainServer) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *messages.Started:
		ms.startedProcesses.Add(context.Sender())
		if ms.startedProcesses.Len() == ms.processCount {
			log.Println("Main server: starting broadcast")
			ms.simulate(context)
		}
	}
}
