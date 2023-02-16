package main

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"math/rand"
	"stochastic-checking-simulation/impl/messages"
)

type MainServer struct {
	currPid              *actor.PID
	expectedProcessCount int
	startedProcesses     *actor.PIDSet
}

func (ms *MainServer) InitMainServer(currPid *actor.PID, processCount int) {
	ms.currPid = currPid
	ms.expectedProcessCount = processCount
	ms.startedProcesses = actor.NewPIDSet()
}

func (ms *MainServer) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *messages.Started:
		ms.startedProcesses.Add(context.Sender())
		if ms.startedProcesses.Len() == ms.expectedProcessCount {
			fmt.Println("Main server: starting broadcast")
			for _, pid := range ms.startedProcesses.Values() {
				msg := &messages.Broadcast{
					Value: int64(rand.Int()),
				}
				context.RequestWithCustomSender(pid, msg, ms.currPid)
			}
		}
	}
}
