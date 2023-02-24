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

func (s *TransactionManager) SendOutTransaction(context actor.Context, p protocols.Process) {
	if s.TransactionsToSendOut > 0 {
		p.Broadcast(context, int64(rand.Int()))
		s.TransactionsToSendOut--

		context.ReenterAfter(
			actor.NewFuture(context.ActorSystem(), config.TimeoutToSendOutNewTransactionNs),
			func(res interface{}, err error) {
				s.SendOutTransaction(context, p)
			})
	}
}
