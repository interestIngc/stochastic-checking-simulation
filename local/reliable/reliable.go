package main

import (
	"fmt"
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
		ProcessCount:            processCount,
		FaultyProcesses:         20,
		MinOwnWitnessSetSize:    16,
		MinPotWitnessSetSize:    32,
		OwnWitnessSetRadius:     1900.0,
		PotWitnessSetRadius:     1910.0,
		WitnessThreshold:        4,
		RecoverySwitchTimeoutNs: 1000000,
		NodeIdSize:              256,
		NumberOfBins:            32,
	}

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8080)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	pids := make([]*actor.PID, processCount)
	processes := make([]*reliable.Process, processCount)

	logger := log.Default()

	mainserver, e := system.Root.SpawnNamed(
		actor.PropsFromFunc(func(c actor.Context) {}),
		"mainserver",
	)
	if e != nil {
		logger.Fatal("Could not spawn the mainserver")
	}

	for i := 0; i < processCount; i++ {
		pids[i] = actor.NewPID("127.0.0.1:8080", fmt.Sprintf("process%d", i))
	}

	for i := 0; i < processCount; i++ {
		process := &reliable.Process{}
		processes[i] = process
		process.InitProcess(
			pids[i],
			pids,
			parameters,
			logger,
			protocols.NewTransactionManager(1, 1),
			mainserver,
		)

		_, e :=
			system.Root.SpawnNamed(
				actor.PropsFromProducer(
					func() actor.Actor {
						return process
					}),
				pids[i].Id,
			)
		if e != nil {
			logger.Fatal(fmt.Sprintf("Could not spawn process %d: %e", i, e))
		}
	}

	processes[0].Broadcast(system.Root, 5)

	_, _ = console.ReadLine()
}
