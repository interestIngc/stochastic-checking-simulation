package main

import (
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"log"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/protocols/accountability/reliable"
)

func main() {
	processCount := 256
	parameters := &parameters.Parameters{
		FaultyProcesses:         20,
		MinOwnWitnessSetSize:    16,
		MinPotWitnessSetSize:    32,
		OwnWitnessSetRadius:     1900.0,
		PotWitnessSetRadius:     1910.0,
		WitnessThreshold:        4,
		RecoverySwitchTimeoutNs: 1000000000,
		NodeIdSize:              256,
		NumberOfBins:            32,
	}

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8080)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	pids := make([]*actor.PID, processCount)
	processes := make([]*reliable.Process, processCount)

	for i := 0; i < processCount; i++ {
		processes[i] = &reliable.Process{}
		pids[i] =
			system.Root.Spawn(
				actor.PropsFromProducer(
					func() actor.Actor {
						return processes[i]
					}),
			)
	}

	logger := log.Default()
	for i := 0; i < processCount; i++ {
		processes[i].InitProcess(
			pids[i],
			pids,
			parameters,
			logger,
			protocols.NewTransactionManager(1, 1),
		)
	}

	processes[0].Broadcast(system.Root, 5)

	_, _ = console.ReadLine()
}
