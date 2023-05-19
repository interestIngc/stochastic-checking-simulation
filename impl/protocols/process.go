package protocols

import (
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
)

type Process interface {
	InitProcess(
		processIndex int32,
		actorPids []string,
		parameters *parameters.Parameters,
		context *context.ReliableContext,
		eventLogger *eventlogger.EventLogger,
		ownDeliveredTransactions chan bool,
		sendOwnDeliveredTransactions bool,
	)

	HandleMessage(
		sender int32,
		message *messages.BroadcastInstanceMessage,
	)

	Broadcast(value int32)
}
