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
	nameServerAddr = flag.String("nameserver", "", "address of the name server, e.g. 127.0.0.1:8080")
)

func main() {
	flag.Parse()

	host, portStr, e := net.SplitHostPort(*nameServerAddr)
	if e != nil {
		fmt.Printf("Could not split %s into host and port\n", *nameServerAddr)
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

	server := &NameServer{}
	pid, e := system.Root.SpawnNamed(
		actor.PropsFromProducer(
			func() actor.Actor {
				return server
			}),
			"nameserver",
	)
	if e != nil {
		fmt.Printf("Could not start a name server: %s\n", e)
		return
	}

	server.InitNameServer(pid)
	fmt.Printf("Nameserver started at: %s\n", *nameServerAddr)

	_, _ = console.ReadLine()
}
