package main

import (
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"log"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/protocols/scalable"
)

func main() {
	processCount := 256
	parameters := &parameters.Parameters{
		ProcessCount:       processCount,
		GossipSampleSize:   20,
		EchoSampleSize:     16,
		EchoThreshold:      10,
		ReadySampleSize:    20,
		ReadyThreshold:     10,
		DeliverySampleSize: 20,
		DeliveryThreshold:  15,
	}

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8080)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	pids := make([]*actor.PID, processCount)
	processes := make([]*scalable.Process, processCount)

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
		process := &scalable.Process{}
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
