package main

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"math/rand"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/messages"
)

type NameServer struct {
	currPid *actor.PID
	startedProcesses *actor.PIDSet
}

func (ns *NameServer) InitNameServer(currPid *actor.PID) {
	ns.currPid = currPid
	ns.startedProcesses = actor.NewPIDSet()
}

func (ns *NameServer) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *messages.Started:
		ns.startedProcesses.Add(context.Sender())
		if ns.startedProcesses.Len() == config.ProcessCount {
			fmt.Println("Nameserver: starting broadcast")
			for _, pid := range ns.startedProcesses.Values() {
				msg := &messages.Broadcast{
					Value: int64(rand.Int()),
				}
				context.RequestWithCustomSender(pid, msg, ns.currPid)
			}
		}
	}
}
