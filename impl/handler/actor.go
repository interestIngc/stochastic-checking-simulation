package handler

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"math/rand"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"time"
)

type Actor struct {
	processIndex int64

	context *context.ReliableContext

	actorPids  []*actor.PID
	mainServer *actor.PID

	transactionsToSendOut    int
	transactionInitTimeoutNs int

	process protocols.Process
}

func (a *Actor) InitActor(
	processIndex int64,
	actorPids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger,
	mainServer *actor.PID,
	transactionsToSendOut int,
	transactionInitTimeoutNs int,
	process protocols.Process,
) {
	n := len(actorPids)

	a.processIndex = processIndex

	a.actorPids = make([]*actor.PID, n+1)
	for i := 0; i < n; i++ {
		a.actorPids[i] = actorPids[i]
	}
	a.actorPids[n] = mainServer
	a.mainServer = mainServer

	a.transactionsToSendOut = transactionsToSendOut
	a.transactionInitTimeoutNs = transactionInitTimeoutNs

	a.context = &context.ReliableContext{}
	a.context.InitContext(processIndex, n+1, logger, parameters.RetransmissionTimeoutNs)

	a.process = process
	a.process.InitProcess(processIndex, actorPids, parameters, a.context.Logger)
}

func (a *Actor) Receive(context actor.Context) {
	switch message := context.Message().(type) {
	case *actor.Started:
		a.context.Logger.OnStart()
		msg := &messages.Started{}
		a.context.Logger.Println(fmt.Sprintf("Sent message to mainserver: %s", msg.ToString()))

		startedMessage := a.context.MakeNewMessage()
		startedMessage.Content = &messages.Message_Started{
			Started: msg,
		}
		a.context.Send(context, a.mainServer, startedMessage)
	case *actor.Stop:
		a.context.Logger.OnStop()
	case *messages.Ack:
		a.context.OnAck(message.Stamp)
	case *messages.Message:
		sender := message.Sender
		stamp := message.Stamp

		a.context.Logger.OnMessageReceived(message.Sender, stamp)

		a.context.SendAck(context, a.actorPids[sender], sender, stamp)

		if a.context.ReceivedMessage(sender, stamp) {
			return
		}

		switch content := message.Content.(type) {
		case *messages.Message_Simulate:
			a.context.Logger.OnSimulationStart()
			a.Simulate(context)
		case *messages.Message_BroadcastInstanceMessage:
			a.process.HandleMessage(context, a.context, sender, content.BroadcastInstanceMessage)
		}
	}
}

func (a *Actor) Simulate(actorContext actor.Context) {
	if a.transactionsToSendOut > 0 {
		a.process.Broadcast(actorContext, a.context, int64(rand.Int()))
		a.transactionsToSendOut--

		actorContext.ReenterAfter(
			actor.NewFuture(actorContext.ActorSystem(), time.Duration(a.transactionInitTimeoutNs)),
			func(res interface{}, err error) {
				a.Simulate(actorContext)
			})
	}
}
