package utils

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"os"
	"path/filepath"
	"time"
)

func MakeCustomPid(pid *actor.PID) string {
	return fmt.Sprintf("%s,%s", pid.Address, pid.Id)
}

func JoinIpAndPort(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
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
