package main

import (
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"log"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/protocols/scalable"
)

func main() {
	processCount := 256
	parameters := &config.Parameters{
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

	for i := 0; i < processCount; i++ {
		processes[i] = &scalable.Process{}
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
		processes[i].InitProcess(pids[i], pids, parameters, logger)
	}

	processes[0].Broadcast(system.Root, 5)

	_, _ = console.ReadLine()
}
