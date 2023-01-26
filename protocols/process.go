package protocols

import "github.com/asynkron/protoactor-go/actor"

type Process interface {
	Broadcast(context actor.SenderContext, value int64)
}

type FaultyProcess interface {
	Broadcast(context actor.SenderContext, value int64)
	FaultyBroadcast(context actor.SenderContext, value1 int64, value2 int64)
}

type ValueType int64
