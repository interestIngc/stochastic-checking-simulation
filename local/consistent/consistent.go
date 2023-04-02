package main

import (
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"log"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/protocols/accountability/consistent"
)

func main() {
	processCount := 256
	faultyProcessesCount := 20
	parameters := &parameters.Parameters{
		ProcessCount:            processCount,
		FaultyProcesses:         faultyProcessesCount,
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
	processes := make([]protocols.Process, processCount)
	correctProcesses := make([]*consistent.CorrectProcess, processCount-faultyProcessesCount)
	faultyProcesses := make([]*consistent.FaultyProcess, faultyProcessesCount)

	for i := 0; i < faultyProcessesCount; i++ {
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

	for i := 0; i < processCount-faultyProcessesCount; i++ {
		process := &consistent.CorrectProcess{}
		correctProcesses[i] = process
		processes[i+faultyProcessesCount] = process
		pids[i+faultyProcessesCount] =
			system.Root.Spawn(
				actor.PropsFromProducer(
					func() actor.Actor {
						return process
					}),
			)
	}

	logger := log.Default()
	for i := 0; i < faultyProcessesCount; i++ {
		faultyProcesses[i].InitProcess(
			pids[i],
			pids,
			parameters,
			logger,
			protocols.NewTransactionManager(1, 1),
		)
	}
	for i := 0; i < processCount-faultyProcessesCount; i++ {
		correctProcesses[i].InitProcess(
			pids[i+faultyProcessesCount],
			pids,
			parameters,
			logger,
			protocols.NewTransactionManager(1, 1),
		)
	}

	faultyProcesses[0].FaultyBroadcast(system.Root, 0, 5)

	_, _ = console.ReadLine()
}
