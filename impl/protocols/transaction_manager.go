package protocols

import (
	"github.com/asynkron/protoactor-go/actor"
	"math/rand"
	"time"
)

type TransactionManager struct {
	transactionsToSendOut    int
	transactionInitTimeoutNs int
}

func NewTransactionManager(
	transactionsToSendOut int,
	transactionInitTimeoutNs int,
) *TransactionManager {
	t := new(TransactionManager)
	t.transactionsToSendOut = transactionsToSendOut
	t.transactionInitTimeoutNs = transactionInitTimeoutNs
	return t
}

func (tm *TransactionManager) Simulate(context actor.Context, p Process) {
	if tm.transactionsToSendOut > 0 {
		p.Broadcast(context, int64(rand.Int()))
		tm.transactionsToSendOut--

		context.ReenterAfter(
			actor.NewFuture(context.ActorSystem(), time.Duration(tm.transactionInitTimeoutNs)),
			func(res interface{}, err error) {
				tm.Simulate(context, p)
			})
	}
}
