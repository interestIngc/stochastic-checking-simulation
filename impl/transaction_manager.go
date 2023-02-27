package impl

import (
	"github.com/asynkron/protoactor-go/actor"
	"math/rand"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/protocols"
)

type TransactionManager struct {
	TransactionsToSendOut int64
}

func (tm *TransactionManager) SendOutTransaction(context actor.Context, p protocols.Process) {
	if tm.TransactionsToSendOut > 0 {
		p.Broadcast(context, int64(rand.Int()))
		tm.TransactionsToSendOut--

		context.ReenterAfter(
			actor.NewFuture(context.ActorSystem(), config.TimeoutToSendOutNewTransactionNs),
			func(res interface{}, err error) {
				tm.SendOutTransaction(context, p)
			})
	}
}
