package protocols

import (
	"github.com/asynkron/protoactor-go/actor"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
)

type Process interface {
	HandleMessage(
		actorContext actor.Context,
		context *context.ReliableContext,
		sender int64,
		message *messages.BroadcastInstanceMessage)

	InitProcess(
		processIndex int64,
		actorPids []*actor.PID,
		parameters *parameters.Parameters,
		logger *eventlogger.EventLogger,
	)

	Broadcast(context actor.Context, reliableContext *context.ReliableContext, value int64)
}
