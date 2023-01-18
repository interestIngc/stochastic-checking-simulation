package main

import (
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"stochastic-checking-simulation/protocols/accountability/consistent"
)

func main() {
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8081)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	otherPid := actor.NewPID("127.0.0.1:8080", "pid1")

	process := &consistent.CorrectProcess{}
	currPid, e :=
		system.Root.SpawnNamed(
			actor.PropsFromProducer(
				func() actor.Actor {
					return process
				}),
			"pid1",
		)
	if e != nil {
		fmt.Printf("Error while generating pid happened: %s\n", e)
		return
	}

	pids := make([]*actor.PID, 2)
	pids[0] = otherPid
	pids[1] = currPid

	process.InitCorrectProcess(currPid, pids)

	_, _ = console.ReadLine()
}
