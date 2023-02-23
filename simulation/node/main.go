package main

import (
	"bufio"
	"flag"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"log"
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
	filePath     = flag.String("file", "", "absolute path to the input file")
	processIndex = flag.Int("i", 0, "index of the current process in the system")
)

const Bytes = 4

func exit(message string) {
	log.Println(message)
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
		ProcessCount:            parseInt(parameters["n"]),
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

func joinWithPort(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

func main() {
	flag.Parse()

	file, e := os.Open(*filePath)
	if e != nil {
		log.Printf("Can't read from file %s", *filePath)
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

	parameters := getParameters(parametersMap)

	ipBytes := make([]int, Bytes)

	for i, currByte := range strings.Split(config.BaseIpAddress, ".") {
		ipBytes[i], e = strconv.Atoi(currByte)
		if e != nil {
			exit(fmt.Sprintf("Byte %d in base ip address is invalid", i))
		}
		if i >= Bytes {
			exit("base ip address is not in ipv4")
		}
	}

	//var processIp string
	var processPort int
	pids := make([]*actor.PID, parameters.ProcessCount)

	port := config.Port

	for i := 0; i < parameters.ProcessCount; i++ {
		port++
		leftByteInd := Bytes - 1
		for ; leftByteInd >= 0 && ipBytes[leftByteInd] == 255; leftByteInd-- {
		}
		if leftByteInd == -1 {
			exit(
				"cannot assign ip addresses, number of processes in the system is too high")
		}
		ipBytes[leftByteInd]++
		for ind := leftByteInd + 1; ind < Bytes; ind++ {
			ipBytes[ind] = 0
		}

		ipAsStr := make([]string, Bytes)
		for ind := 0; ind < Bytes; ind++ {
			ipAsStr[ind] = strconv.Itoa(ipBytes[ind])
		}

		//currIp := strings.Join(ipAsStr, ".")
		if i == *processIndex {
			//processIp = currIp
			processPort = port
		}
		//pids[i] = actor.NewPID(joinWithPort(currIp, config.Port), "pid")
		pids[i] = actor.NewPID(joinWithPort(config.BaseIpAddress, port), "pid")
	}

	mainServer := actor.NewPID(joinWithPort(config.BaseIpAddress, config.Port), "mainserver")

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
	//remoteConfig := remote.Configure(processIp, config.Port)
	remoteConfig := remote.Configure(config.BaseIpAddress, processPort)
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
	log.Printf("%s: started\n", utils.MakeCustomPid(currPid))

	system.Root.RequestWithCustomSender(mainServer, &messages.Started{}, currPid)

	_, _ = console.ReadLine()
}
