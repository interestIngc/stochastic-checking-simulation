package main

import "github.com/asynkron/protoactor-go/actor"

type Process interface {
	Receive(actor.Context)
}

type ValueType int64
