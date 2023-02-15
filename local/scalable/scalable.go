package main

import (
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/protocols/scalable"
)

func main() {
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8080)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	pids := make([]*actor.PID, config.ProcessCount)
	processes := make([]*scalable.Process, config.ProcessCount)

	for i := 0; i < config.ProcessCount; i++ {
		processes[i] = &scalable.Process{}
		pids[i] =
			system.Root.Spawn(
				actor.PropsFromProducer(
					func() actor.Actor {
						return processes[i]
					}),
			)
	}
	for i := 0; i < config.ProcessCount; i++ {
		processes[i].InitProcess(pids[i], pids)
	}

	processes[0].Broadcast(system.Root, 5)

	_, _ = console.ReadLine()
}
