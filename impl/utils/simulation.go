package utils

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"math"
	"os"
	"path/filepath"
	"stochastic-checking-simulation/impl/messages"
	"strconv"
	"strings"
	"time"
)

const Bytes = 4

func JoinIpAndPort(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

func GenerateStarTopology(nodes int, baseIp string, logger *log.Logger) []string {
	ipBytes := parseIpv4(baseIp, logger)

	nodeIps := make([]string, nodes)
	nodeIps[0] = baseIp

	for i := 1; i < nodes; i++ {
		leftByteInd := Bytes - 1
		for ; leftByteInd >= 0 && ipBytes[leftByteInd] == 255; leftByteInd-- {
		}
		if leftByteInd == -1 {
			logger.Fatal("Cannot assign ip addresses, number of processes in the system is too high")
		}
		ipBytes[leftByteInd]++
		for ind := leftByteInd + 1; ind < Bytes; ind++ {
			ipBytes[ind] = 0
		}

		nodeIps[i] = convertIpByteArrayToStr(ipBytes)
	}

	return nodeIps
}

func GenerateTreeTopology(
	numberOfChildren int,
	nodes int,
	baseIp string,
	logger *log.Logger,
) []string {
	nodeIps := make([]string, nodes)

	currIp := parseIpv4(baseIp, logger)

	for i := 0; i < nodes; i++ {
		endIp := byte(i + 1)
		lowSubnet := byte(i / numberOfChildren)
		upSubnet := byte(i / (int(math.Pow(float64(numberOfChildren), 2))))

		currIp[3] = endIp
		currIp[2] = lowSubnet
		currIp[1] = upSubnet
		nodeIps[i] = convertIpByteArrayToStr(currIp)
	}

	return nodeIps
}

func GeneratePids(
	nodeIps []string,
	basePort int,
	processCount int,
) []string {
	pids := make([]string, processCount)

	nodes := len(nodeIps)

	for i := 0; i < processCount; i++ {
		pids[i] = JoinIpAndPort(nodeIps[i%nodes], basePort+i)
	}

	return pids
}

func OpenLogFile(logFile string) *os.File {
	dir, _ := filepath.Split(logFile)
	e := os.MkdirAll(dir, os.ModePerm)
	if e != nil {
		log.Printf("Could not create parent directories for %s, error: %e", logFile, e)
	}

	f, e := os.Create(logFile)
	if e != nil {
		log.Printf("Could not open file %s to write logs into, error: %e", logFile, e)
	}

	return f
}

func GetNow() int64 {
	return time.Now().UnixNano()
}

func Unmarshal(data []byte) (*messages.Message, error) {
	msg := &messages.Message{}

	err := proto.Unmarshal(data, msg)
	if err != nil {
		log.Printf("Could not unmarshal message: %v", data)
		return nil, err
	}
	return msg, nil
}

func Marshal(message *messages.Message) ([]byte, error) {
	data, e := proto.Marshal(message)
	if e != nil {
		log.Printf("Could not marshal message, %e", e)
	}
	return data, e
}

func parseIpv4(ip string, logger *log.Logger) []byte {
	splittedIp := strings.Split(ip, ".")
	if len(splittedIp) != Bytes {
		logger.Fatal("Ip address %s is invalid", ip)
	}

	ipBytes := make([]byte, Bytes)
	for i, currByte := range splittedIp {
		byteAsInt, e := strconv.Atoi(currByte)
		if e != nil || byteAsInt < 0 || byteAsInt > 255 {
			logger.Fatalf("Ip address %s is invalid, byte at index %d is %s", ip, i, currByte)
		}
		ipBytes[i] = byte(byteAsInt)
	}

	return ipBytes
}

func convertIpByteArrayToStr(ipBytes []byte) string {
	ipBytesAsStr := make([]string, Bytes)
	for ind := 0; ind < Bytes; ind++ {
		ipBytesAsStr[ind] = strconv.Itoa(int(ipBytes[ind]))
	}
	return strings.Join(ipBytesAsStr, ".")
}
