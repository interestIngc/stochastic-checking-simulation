package consistent

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"math"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/hashing"
	"stochastic-checking-simulation/messages"
	"stochastic-checking-simulation/protocols"
	"stochastic-checking-simulation/utils"
)

type messageState struct {
	receivedEcho  map[string]bool
	echoCount     map[protocols.ValueType]int
	witnessSet    map[string]bool
}

func newMessageState() *messageState {
	ms := new(messageState)
	ms.receivedEcho = make(map[string]bool)
	ms.echoCount = make(map[protocols.ValueType]int)
	return ms
}

type CorrectProcess struct {
	currPid          *actor.PID
	pids             map[string]*actor.PID
	msgCounter       int64
	acceptedMessages map[string]map[int64]protocols.ValueType
	messagesLog      map[string]map[int64]*messageState
	wSelector        *hashing.WitnessesSelector
	historyHash      *hashing.HistoryHash
}

func (p *CorrectProcess) InitProcess(currPid *actor.PID, pids []*actor.PID) {
	p.currPid = currPid
	p.msgCounter = 0
	p.pids = make(map[string]*actor.PID)
	p.acceptedMessages = make(map[string]map[int64]protocols.ValueType)
	p.messagesLog = make(map[string]map[int64]*messageState)

	ids := make([]string, len(pids))
	for i, pid := range pids {
		id := utils.PidToString(pid)
		ids[i] = id
		p.pids[id] = pid
		p.acceptedMessages[id] = make(map[int64]protocols.ValueType)
		p.messagesLog[id] = make(map[int64]*messageState)
	}

	var hasher hashing.Hasher
	if config.NodeIdSize == 256 {
		hasher = hashing.HashSHA256{}
	} else {
		hasher = hashing.HashSHA512{}
	}

	p.wSelector = &hashing.WitnessesSelector{NodeIds: ids, Hasher: hasher}
	binCapacity := uint(math.Pow(2, float64(config.NodeIdSize / config.NumberOfBins)))
	p.historyHash = hashing.NewHistoryHash(uint(config.NumberOfBins), binCapacity, hasher)
}

func (p *CorrectProcess) broadcast(context actor.SenderContext, message *messages.ProtocolMessage) {
	for _, pid := range p.pids {
		context.RequestWithCustomSender(pid, message, p.currPid)
	}
}

func (p *CorrectProcess) verify(
	context actor.SenderContext, sender string, msg *messages.ProtocolMessage) bool {
	value := protocols.ValueType(msg.Value)
	msgState := p.messagesLog[msg.Author][msg.SeqNumber]

	acceptedValue, accepted := p.acceptedMessages[msg.Author][msg.SeqNumber]
	if accepted {
		if acceptedValue != value {
			//fmt.Printf("%s: Detected a duplicated seq number attack\n", p.currPid.Address)
			return false
		}
	} else if msgState != nil {
		if msgState.witnessSet[sender] && !msgState.receivedEcho[sender] {
			msgState.receivedEcho[sender] = true
			msgState.echoCount[value]++
			if msgState.echoCount[value] >= config.WitnessThreshold {
				p.acceptedMessages[msg.Author][msg.SeqNumber] = value
				p.historyHash.Insert(utils.TransactionToBytes(msg.Author, msg.SeqNumber))
				delete(p.messagesLog[msg.Author], msg.SeqNumber)

				fmt.Printf(
					"%s: Accepted transaction with seq number %d and value %d from %s\n",
					utils.PidToString(p.currPid), msg.SeqNumber, msg.Value, msg.Author)
			}
		}
	} else {
		msgState := newMessageState()
		msgState.witnessSet, _ = p.wSelector.GetWitnessSet(msg.Author, msg.SeqNumber, p.historyHash)
		p.messagesLog[msg.Author][msg.SeqNumber] = msgState

		message := &messages.ProtocolMessage{
			Stage: messages.ProtocolMessage_VERIFY,
			Author: msg.Author,
			SeqNumber: msg.SeqNumber,
			Value: msg.Value,
		}
		for id := range msgState.witnessSet {
			context.RequestWithCustomSender(p.pids[id], message, p.currPid)
		}
	}
	return true
}

func (p *CorrectProcess) Receive(context actor.Context) {
	message := context.Message()

	switch message.(type) {
	case *messages.Broadcast:
		msg := message.(*messages.Broadcast)
		p.Broadcast(context, msg.Value)
	case *messages.ProtocolMessage:
		msg := message.(*messages.ProtocolMessage)
		senderId := utils.PidToString(context.Sender())

		doBroadcast := p.verify(context, senderId, msg)

		if msg.Stage == messages.ProtocolMessage_VERIFY && doBroadcast {
			p.broadcast(
				context,
				&messages.ProtocolMessage{
					Stage: messages.ProtocolMessage_ECHO,
					Author: msg.Author,
					SeqNumber: msg.SeqNumber,
					Value: msg.Value,
				})
		}
	}
}

func (p *CorrectProcess) Broadcast(context actor.SenderContext, value int64) {
	id := utils.PidToString(p.currPid)
	msg := &messages.ProtocolMessage{
		Stage: messages.ProtocolMessage_ECHO,
		Author: id,
		SeqNumber: p.msgCounter,
		Value: value,
	}
	p.verify(context, id, msg)
	p.msgCounter++
}
