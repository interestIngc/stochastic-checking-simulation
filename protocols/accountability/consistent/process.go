package consistent

import "github.com/asynkron/protoactor-go/actor"

type Process interface {
	Receive(actor.Context)
}

type ValueType int32

type MessageState struct {
	receivedEcho  map[*actor.PID]bool
	echoCount     map[ValueType]int
	witnessSet    []*actor.PID
}

func NewMessageState() *MessageState {
	ms := new(MessageState)
	ms.receivedEcho = make(map[*actor.PID]bool)
	ms.echoCount = make(map[ValueType]int)
	return ms
}
