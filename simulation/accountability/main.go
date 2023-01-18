package main

import (
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/protocols/accountability/consistent"
)

func main() {
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8080)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	pids := make([]*actor.PID, config.ProcessCount)
	processes := make([]consistent.Process, config.ProcessCount)
	correctProcesses := make([]*consistent.CorrectProcess, config.ProcessCount - config.FaultyProcesses)
	faultyProcesses := make([]*consistent.FaultyProcess, config.FaultyProcesses)

	for i := 0; i < config.ProcessCount; i++ {
		if i < config.FaultyProcesses {
			faultyProcesses[i] = &consistent.FaultyProcess{}
			processes[i] = faultyProcesses[i]
		} else {
			correctProcesses[i - config.FaultyProcesses] = &consistent.CorrectProcess{}
			processes[i] = correctProcesses[i - config.FaultyProcesses]
		}
		pids[i] =
			system.Root.Spawn(
				actor.PropsFromProducer(
					func() actor.Actor {
						return processes[i]
					}),
			)
	}
	for i := 0; i < config.FaultyProcesses; i++ {
		faultyProcesses[i].InitFaultyProcess(pids[i], pids)
	}
	for i := 0; i < config.ProcessCount - config.FaultyProcesses; i++ {
		correctProcesses[i].InitCorrectProcess(pids[i + config.FaultyProcesses], pids)
	}

	faultyProcesses[0].Broadcast(system.Root, 0, 5)

	_, _ = console.ReadLine()
}
