package consistent

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/hashing"
	"stochastic-checking-simulation/messages"
)

type CorrectProcess struct {
	currPid          *actor.PID
	pids             []*actor.PID
	msgCounter       int32
	acceptedMessages map[*actor.PID]map[int32]ValueType
	messagesLog      map[*actor.PID]map[int32]*MessageState
	wSelector        *hashing.WitnessesSelector
	historyHash      *hashing.HistoryHash
}

func (p *CorrectProcess) InitCorrectProcess(currPid *actor.PID, pids []*actor.PID) {
	p.currPid = currPid
	p.pids = pids
	p.msgCounter = 0
	p.acceptedMessages = make(map[*actor.PID]map[int32]ValueType)
	p.messagesLog = make(map[*actor.PID]map[int32]*MessageState)

	for _, pid := range pids {
		p.acceptedMessages[pid] = make(map[int32]ValueType)
		p.messagesLog[pid] = make(map[int32]*MessageState)
	}

	var hasher hashing.Hasher
	if config.CryptoAlgorithm == "sha256" {
		hasher = hashing.HashSHA256{}
	} else {
		hasher = hashing.HashSHA512{}
	}

	p.wSelector = &hashing.WitnessesSelector{NodeIds: pids, Hasher: hasher}
	binNum := uint(len(pids) / 8)
	p.historyHash = hashing.NewHistoryHash(binNum, 256, hasher)
}

func (p *CorrectProcess) broadcast(context actor.SenderContext, message *messages.ProtocolMessage) {
	for _, pid := range p.pids {
		context.RequestWithCustomSender(pid, message, p.currPid)
	}
}

func (p *CorrectProcess) verify(
	context actor.SenderContext, senderId *actor.PID, msg *messages.ProtocolMessage) bool {
	value := ValueType(msg.Value)
	msgState := p.messagesLog[msg.Author][msg.SeqNumber]
	acceptedValue, accepted := p.acceptedMessages[msg.Author][msg.SeqNumber]
	if accepted {
		if acceptedValue != value {
			//fmt.Printf("%s: Detected a duplicated seq number attack\n", p.currPid.Id)
			return false
		}
	} else if msgState != nil {
		if !msgState.receivedEcho[senderId] {
			msgState.receivedEcho[senderId] = true
			msgState.echoCount[value]++
			if msgState.echoCount[value] >= config.WitnessThreshold {
				p.acceptedMessages[msg.Author][msg.SeqNumber] = value
				p.historyHash.Insert(
					hashing.TransactionToBytes(msg.Author, msg.SeqNumber))
				p.messagesLog[msg.Author][msg.SeqNumber] = nil
				fmt.Printf(
					"%s: Accepted transaction with seq number %d and value %d from %s\n",
					p.currPid.Id, msg.SeqNumber, msg.Value, msg.Author.Id)
			}
		}
	} else {
		msgState := NewMessageState()
		msgState.witnessSet = p.wSelector.GetWitnessSet(msg.Author, msg.SeqNumber, p.historyHash)
		p.messagesLog[msg.Author][msg.SeqNumber] = msgState

		message := &messages.ProtocolMessage{
			Stage: messages.ProtocolMessage_VERIFY,
			Author: msg.Author,
			SeqNumber: msg.SeqNumber,
			Value: msg.Value,
		}
		for _, w := range msgState.witnessSet {
			context.RequestWithCustomSender(w, message, p.currPid)
		}
	}
	return true
}

func (p *CorrectProcess) Receive(context actor.Context) {
	msg, ok := context.Message().(*messages.ProtocolMessage)
	if !ok {
		return
	}
	senderId := context.Sender()

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

func (p *CorrectProcess) Broadcast(context actor.SenderContext, value int32) {
	msg := &messages.ProtocolMessage{
		Stage: messages.ProtocolMessage_ECHO,
		Author: p.currPid,
		SeqNumber: p.msgCounter,
		Value: value,
	}
	p.verify(context, p.currPid, msg)
	p.msgCounter++
}
