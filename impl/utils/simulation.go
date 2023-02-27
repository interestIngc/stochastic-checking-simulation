package utils

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"os"
)

func MakeCustomPid(pid *actor.PID) string {
	return fmt.Sprintf("Address:%s,Id:%s", pid.Address, pid.Id)
}

func ExitWithError(logger *log.Logger, errorMessage string) {
	logger.Println(errorMessage)
	os.Exit(1)
}

func OpenLogFile(logFile string) *os.File {
	f, e := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if e != nil {
		log.Printf("Could not open file %s to write logs into", logFile)
	}
	return f
}
