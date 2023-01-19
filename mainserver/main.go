package main

import (
	"flag"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"net"
	"strconv"
)

var (
	mainServerAddr = flag.String("mainserver", "", "address of the main server, e.g. 127.0.0.1:8080")
)

func main() {
	flag.Parse()

	host, portStr, e := net.SplitHostPort(*mainServerAddr)
	if e != nil {
		fmt.Printf("Could not split %s into host and port\n", *mainServerAddr)
		return
	}
	port, e := strconv.Atoi(portStr)
	if e != nil {
		fmt.Printf("Could not convert port string representation into int: %s\n", e)
		return
	}

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(host, port)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	server := &MainServer{}
	pid, e := system.Root.SpawnNamed(
		actor.PropsFromProducer(
			func() actor.Actor {
				return server
			}),
			"mainserver",
	)
	if e != nil {
		fmt.Printf("Could not start a main server: %s\n", e)
		return
	}

	server.InitMainServer(pid)
	fmt.Printf("Main server started at: %s\n", *mainServerAddr)

	_, _ = console.ReadLine()
}
