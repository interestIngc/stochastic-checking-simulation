package protocols

import (
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
)

// Process interface represents a process executing a reliable broadcast protocol.
// It exports three methods:
// InitProcess is a constructor for an instance of Process
// HandleMessage handles an incoming message, potentially
// creating new messages to be sent to other processes in the system.
// Broadcast initiates broadcast of a new message with current process as the source.
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
