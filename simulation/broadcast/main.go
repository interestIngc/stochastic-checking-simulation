package main

import (
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/protocols/broadcast"
)

func main() {
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8080)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	pids := make([]*actor.PID, config.ProcessCount)
	processes := make([]*broadcast.Process, config.ProcessCount)

	for i := 0; i < config.ProcessCount; i++ {
		processes[i] = &broadcast.Process{}
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

	processes[0].Broadcast(system.Root, int32(5))

	//for seq := 0; seq < 4; seq++ {
	//	for i := 0; i < config.ProcessCount; i++ {
	//		processes[i].Broadcast(system.Root, int32(i))
	//	}
	//}

	//for i := 1; i < 2; i++ {
	//	system.Root.RequestWithCustomSender(
	//		pids[i],
	//		&messages.Message{
	//			Stage:     messages.Message_INITIAL,
	//			Author:    pids[0],
	//			SeqNumber: 0,
	//			Value:     1,
	//		},
	//		pids[0])
	//}
	//for i := 2; i < 6; i++ {
	//	system.Root.RequestWithCustomSender(
	//		pids[i],
	//		&messages.Message{
	//			Stage:     messages.Message_INITIAL,
	//			Author:    pids[0],
	//			SeqNumber: 0,
	//			Value:     2,
	//		},
	//		pids[0])
	//}
	_, _ = console.ReadLine()
}
