package protocols

import (
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
)

type Process interface {
	HandleMessage(
		context *context.ReliableContext,
		sender int32,
		message *messages.BroadcastInstanceMessage)

	InitProcess(
		processIndex int32,
		actorPids []string,
		parameters *parameters.Parameters,
		logger *eventlogger.EventLogger,
		ownDeliveredTransactions chan bool,
	)

	Broadcast(reliableContext *context.ReliableContext, value int32)
}
