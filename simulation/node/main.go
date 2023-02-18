package main

import (
	"bufio"
	"flag"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"net"
	"os"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/protocols/accountability/consistent"
	"stochastic-checking-simulation/impl/protocols/accountability/reliable"
	"stochastic-checking-simulation/impl/protocols/bracha"
	"stochastic-checking-simulation/impl/protocols/scalable"
	"stochastic-checking-simulation/impl/utils"
	"strconv"
	"strings"
	"time"
)

var (
	filePath = flag.String("file", "", "absolute path to the input file")
)

func exit(message string) {
	fmt.Println(message)
	os.Exit(1)
}

func getMandatoryParameter(parameters map[string]string, parameter string) string {
	value, found := parameters[parameter]
	if !found {
		exit(fmt.Sprintf("parameter %s is mandatory\n", parameter))
	}
	return value
}

func parseInt(valueStr string) int {
	if valueStr == "" {
		return 0
	}
	value, e := strconv.Atoi(valueStr)
	if e != nil {
		return 0
	}
	return value
}

func parseFloat(valueStr string) float64 {
	if valueStr == "" {
		return 0
	}
	value, e := strconv.ParseFloat(valueStr, 64)
	if e != nil {
		return 0
	}
	return value
}

func getParameters(parameters map[string]string) *config.Parameters {
	return &config.Parameters{
		FaultyProcesses:         parseInt(parameters["f"]),
		MinOwnWitnessSetSize:    parseInt(parameters["w"]),
		MinPotWitnessSetSize:    parseInt(parameters["v"]),
		OwnWitnessSetRadius:     parseFloat(parameters["wr"]),
		PotWitnessSetRadius:     parseFloat(parameters["vr"]),
		WitnessThreshold:        parseInt(parameters["u"]),
		RecoverySwitchTimeoutNs: time.Duration(parseInt(parameters["recovery_timeout"])),
		NodeIdSize:              parseInt(parameters["node_id_size"]),
		NumberOfBins:            parseInt(parameters["number_of_bins"]),
		GossipSampleSize:        parseInt(parameters["g_size"]),
		EchoSampleSize:          parseInt(parameters["e_size"]),
		EchoThreshold:           parseInt(parameters["e_threshold"]),
		ReadySampleSize:         parseInt(parameters["r_size"]),
		ReadyThreshold:          parseInt(parameters["r_threshold"]),
		DeliverySampleSize:      parseInt(parameters["d_size"]),
		DeliveryThreshold:       parseInt(parameters["d_threshold"]),
	}
}

func main() {
	flag.Parse()

	file, e := os.Open(*filePath)
	if e != nil {
		fmt.Printf("Can't read from file %s", *filePath)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(file)

	parametersMap := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		param := strings.Split(line, " ")
		if len(param) != 2 {
			exit(fmt.Sprintf("Unexpected \"%s\", expected \"parameter_name parameter_value\"\n", line))
		}
		parametersMap[param[0]] = param[1]
	}

	nodes := strings.Split(getMandatoryParameter(parametersMap, "nodes"), ",")
	address := getMandatoryParameter(parametersMap, "current_node")
	host, portStr, e := net.SplitHostPort(address)
	if e != nil {
		exit(fmt.Sprintf("Could not split %s into host and port\n", address))
	}

	port, e := strconv.Atoi(portStr)
	if e != nil {
		exit(fmt.Sprintf("Could not convert port string representation into int: %s\n", e))
	}

	parameters := getParameters(parametersMap)

	pids := make([]*actor.PID, len(nodes))
	for i := 0; i < len(nodes); i++ {
		pids[i] = actor.NewPID(nodes[i], "pid")
	}

	mainServer := actor.NewPID(getMandatoryParameter(parametersMap, "main_server"), "mainserver")

	var process protocols.Process

	protocol := getMandatoryParameter(parametersMap, "protocol")
	switch protocol {
	case "reliable_accountability":
		process = &reliable.Process{}
	case "consistent_accountability":
		process = &consistent.CorrectProcess{}
	case "bracha":
		process = &bracha.Process{}
	case "scalable":
		process = &scalable.Process{}
	default:
		exit(fmt.Sprintf("Invalid protocol: %s\n", protocol))
	}

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure(host, port)
	remoter := remote.NewRemote(system, remoteConfig)
	remoter.Start()

	currPid, e :=
		system.Root.SpawnNamed(
			actor.PropsFromProducer(
				func() actor.Actor {
					return process
				}),
			"pid",
		)
	if e != nil {
		exit(fmt.Sprintf("Error while spawning the process happened: %s\n", e))
	}

	process.InitProcess(currPid, pids, parameters)
	fmt.Printf("%s: started\n", utils.MakeCustomPid(currPid))

	system.Root.RequestWithCustomSender(mainServer, &messages.Started{}, currPid)

	_, _ = console.ReadLine()
}
