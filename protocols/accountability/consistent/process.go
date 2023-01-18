package consistent

import "github.com/asynkron/protoactor-go/actor"

type Process interface {
	Receive(actor.Context)
}

type ValueType int32

type MessageState struct {
	receivedEcho  map[string]bool
	echoCount     map[ValueType]int
	witnessSet    map[string]bool
}

func NewMessageState() *MessageState {
	ms := new(MessageState)
	ms.receivedEcho = make(map[string]bool)
	ms.echoCount = make(map[ValueType]int)
	return ms
}
