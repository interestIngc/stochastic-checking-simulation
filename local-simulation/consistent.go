package main

import (
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/protocols"
	"stochastic-checking-simulation/protocols/accountability/consistent"
)

func main() {
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8080)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	pids := make([]*actor.PID, config.ProcessCount)
	processes := make([]protocols.Process, config.ProcessCount)
	correctProcesses := make([]*consistent.CorrectProcess, config.ProcessCount - config.FaultyProcesses)
	faultyProcesses := make([]*consistent.FaultyProcess, config.FaultyProcesses)

	for i := 0; i < config.FaultyProcesses; i++ {
		process := &consistent.FaultyProcess{}
		faultyProcesses[i] = process
		processes[i] = process
		pids[i] =
			system.Root.Spawn(
				actor.PropsFromProducer(
					func() actor.Actor {
						return process
					}),
			)
	}

	for i := 0; i < config.ProcessCount - config.FaultyProcesses; i++ {
		process := &consistent.CorrectProcess{}
		correctProcesses[i] = process
		processes[i + config.FaultyProcesses] = process
		pids[i + config.FaultyProcesses] =
			system.Root.Spawn(
				actor.PropsFromProducer(
					func() actor.Actor {
						return process
					}),
			)
	}
	for i := 0; i < config.FaultyProcesses; i++ {
		faultyProcesses[i].InitFaultyProcess(pids[i], pids)
	}
	for i := 0; i < config.ProcessCount - config.FaultyProcesses; i++ {
		correctProcesses[i].InitCorrectProcess(pids[i + config.FaultyProcesses], pids)
	}

	faultyProcesses[0].FaultyBroadcast(system.Root, 0, 5)

	_, _ = console.ReadLine()
}
