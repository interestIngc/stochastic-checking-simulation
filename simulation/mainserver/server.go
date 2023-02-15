package main

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"math/rand"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/messages"
)

type MainServer struct {
	currPid          *actor.PID
	startedProcesses *actor.PIDSet
}

func (ns *MainServer) InitMainServer(currPid *actor.PID) {
	ns.currPid = currPid
	ns.startedProcesses = actor.NewPIDSet()
}

func (ns *MainServer) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *messages.Started:
		ns.startedProcesses.Add(context.Sender())
		if ns.startedProcesses.Len() == config.ProcessCount {
			fmt.Println("Main server: starting broadcast")
			for _, pid := range ns.startedProcesses.Values() {
				msg := &messages.Broadcast{
					Value: int64(rand.Int()),
				}
				context.RequestWithCustomSender(pid, msg, ns.currPid)
			}
		}
	}
}
