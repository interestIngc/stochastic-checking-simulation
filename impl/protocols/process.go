package protocols

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"stochastic-checking-simulation/config"
)

type Process interface {
	Receive(context actor.Context)
	InitProcess(currPid *actor.PID, pids []*actor.PID, parameters *config.Parameters, logger *log.Logger)
	Broadcast(context actor.SenderContext, value int64)
}

type FaultyProcess interface {
	Receive(context actor.Context)
	InitProcess(currPid *actor.PID, pids []*actor.PID, parameters *config.Parameters, logger *log.Logger)
	Broadcast(context actor.SenderContext, value int64)
	FaultyBroadcast(context actor.SenderContext, value1 int64, value2 int64)
}

type ValueType int64
