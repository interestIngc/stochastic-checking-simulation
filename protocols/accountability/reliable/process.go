package reliable

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

type WitnessStage int32
const (
	InitialWitnessStage WitnessStage = iota
	SentEchoFromWitness
	SentReadyFromWitness
	SentValidate
)

type Stage int32
const (
	InitialStage Stage = iota
	SentEchoFromProcess
	SentReadyFromProcess
)

type messageState struct {
	echoFromAll        map[string]bool
	readyFromAll       map[string]bool
	readyFromWitnesses map[string]bool
	witnessesValidated map[string]bool

	echoFromAllStat        map[protocols.ValueType]int
	readyFromAllStat       map[protocols.ValueType]int
	readyFromWitnessesStat map[protocols.ValueType]int
	validateMessagesStat   map[protocols.ValueType]int

	stage        Stage
	witnessStage WitnessStage

	ownWitnessSet map[string]bool
	potWitnessSet map[string]bool
}

func newMessageState() *messageState {
	ms := new(messageState)
	ms.echoFromAll = make(map[string]bool)
	ms.readyFromAll = make(map[string]bool)
	ms.readyFromWitnesses = make(map[string]bool)
	ms.witnessesValidated = make(map[string]bool)
	ms.echoFromAllStat = make(map[protocols.ValueType]int)
	ms.readyFromWitnessesStat = make(map[protocols.ValueType]int)
	ms.readyFromAllStat = make(map[protocols.ValueType]int)
	ms.validateMessagesStat = make(map[protocols.ValueType]int)
	ms.stage = InitialStage
	ms.witnessStage = InitialWitnessStage
	return ms
}

type Process struct {
	currPid          *actor.PID
	pids             map[string]*actor.PID
	msgCounter       int64
	acceptedMessages map[string]map[int64]protocols.ValueType
	messagesLog      map[string]map[int64]*messageState

	quorumThreshold        int
	readyMessagesThreshold int

	wSelector   *hashing.WitnessesSelector
	historyHash *hashing.HistoryHash
}

func (p *Process) InitProcess(currPid *actor.PID, pids []*actor.PID) {
	p.currPid = currPid
	p.pids = make(map[string]*actor.PID)
	p.msgCounter = 0

	p.quorumThreshold = int(math.Ceil(float64(config.ProcessCount+config.FaultyProcesses+1) / float64(2)))
	p.readyMessagesThreshold = config.FaultyProcesses + 1

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
	binCapacity := uint(math.Pow(2, float64(config.NodeIdSize/config.NumberOfBins)))
	p.historyHash = hashing.NewHistoryHash(uint(config.NumberOfBins), binCapacity, hasher)
}

func (p *Process) initMessageState(msgData *messages.MessageData) *messageState {
	msgState := p.messagesLog[msgData.Author][msgData.SeqNumber]
	if msgState == nil {
		msgState = newMessageState()
		msgState.ownWitnessSet, msgState.potWitnessSet =
			p.wSelector.GetWitnessSet(msgData.Author, msgData.SeqNumber, p.historyHash)
		p.messagesLog[msgData.Author][msgData.SeqNumber] = msgState
	}
	return msgState
}

func (p *Process) broadcast(context actor.SenderContext, message *messages.ReliableProtocolMessage) {
	for _, pid := range p.pids {
		context.RequestWithCustomSender(pid, message, p.currPid)
	}
}

func (p *Process) broadcastToWitnesses(
	context actor.SenderContext,
	message *messages.ReliableProtocolMessage,
	msgState *messageState) {
	for id := range msgState.potWitnessSet {
		context.RequestWithCustomSender(p.pids[id], message, p.currPid)
	}
}

func (p *Process) broadcastReadyFromWitness(
	context actor.SenderContext,
	msgData *messages.MessageData,
	msgState *messageState) {
	p.broadcast(
		context,
		&messages.ReliableProtocolMessage{
			Stage:       messages.ReliableProtocolMessage_READY_FROM_WITNESS,
			MessageData: msgData,
		})
	msgState.witnessStage = SentReadyFromWitness
}

func (p *Process) isWitness(msgState *messageState) bool {
	return msgState.potWitnessSet[utils.PidToString(p.currPid)]
}

func (p *Process) deliver(msgData *messages.MessageData) {
	p.acceptedMessages[msgData.Author][msgData.SeqNumber] = protocols.ValueType(msgData.Value)
	p.historyHash.Insert(
		utils.TransactionToBytes(msgData.Author, msgData.SeqNumber))
	delete(p.messagesLog[msgData.Author], msgData.SeqNumber)

	fmt.Printf(
		"%s: Accepted transaction with seq number %d and value %d from %s\n",
		utils.PidToString(p.currPid), msgData.SeqNumber, msgData.Value, msgData.Author)
}

func (p *Process) Receive(context actor.Context) {
	message := context.Message()
	switch message.(type) {
	case *messages.Broadcast:
		msg := message.(*messages.Broadcast)
		p.Broadcast(context, msg.Value)
	case *messages.ReliableProtocolMessage:
		msg := message.(*messages.ReliableProtocolMessage)
		msgData := msg.GetMessageData()
		sender := utils.PidToString(context.Sender())
		value := protocols.ValueType(msgData.Value)

		acceptedValue, accepted := p.acceptedMessages[msgData.Author][msgData.SeqNumber]
		if accepted {
			if acceptedValue != value {
				fmt.Printf("%s: Detected a duplicated seq number attack\n", p.currPid.Id)
			}
			return
		}

		msgState := p.initMessageState(msgData)

		switch msg.Stage {
		case messages.ReliableProtocolMessage_INITIAL:
			if !p.isWitness(msgState) || msgState.witnessStage > InitialWitnessStage {
				return
			}
			p.broadcast(
				context,
				&messages.ReliableProtocolMessage{
					Stage:       messages.ReliableProtocolMessage_ECHO_FROM_WITNESS,
					MessageData: msgData,
				})
			msgState.witnessStage = SentEchoFromWitness
		case messages.ReliableProtocolMessage_ECHO_FROM_WITNESS:
			if !msgState.ownWitnessSet[sender] || msgState.stage >= SentEchoFromProcess {
				return
			}
			p.broadcastToWitnesses(
				context,
				&messages.ReliableProtocolMessage{
					Stage:       messages.ReliableProtocolMessage_ECHO_FROM_PROCESS,
					MessageData: msgData,
				},
				msgState)
			msgState.stage = SentEchoFromProcess
		case messages.ReliableProtocolMessage_ECHO_FROM_PROCESS:
			if !p.isWitness(msgState) ||
				msgState.witnessStage >= SentReadyFromWitness ||
				msgState.echoFromAll[sender] {
				return
			}

			msgState.echoFromAll[sender] = true
			msgState.echoFromAllStat[value]++

			if msgState.echoFromAllStat[value] >= p.quorumThreshold {
				p.broadcastReadyFromWitness(context, msgData, msgState)
			}
		case messages.ReliableProtocolMessage_READY_FROM_WITNESS:
			if !msgState.ownWitnessSet[sender] ||
				msgState.stage >= SentReadyFromProcess ||
				msgState.readyFromWitnesses[sender] {
				return
			}

			msgState.readyFromWitnesses[sender] = true
			msgState.readyFromWitnessesStat[value]++

			if msgState.readyFromWitnessesStat[value] >= config.WitnessThreshold {
				p.broadcastToWitnesses(
					context,
					&messages.ReliableProtocolMessage{
						Stage:       messages.ReliableProtocolMessage_READY_FROM_PROCESS,
						MessageData: msgData,
					},
					msgState)
				msgState.stage = SentReadyFromProcess
			}
		case messages.ReliableProtocolMessage_READY_FROM_PROCESS:
			if !p.isWitness(msgState) ||
				msgState.witnessStage == SentValidate ||
				msgState.readyFromAll[sender] {
				return
			}

			msgState.readyFromAll[sender] = true
			msgState.readyFromAllStat[value]++

			if msgState.witnessStage < SentReadyFromWitness &&
				msgState.readyFromAllStat[value] >= p.readyMessagesThreshold {
				p.broadcastReadyFromWitness(context, msgData, msgState)
			}

			if msgState.readyFromAllStat[value] >= p.quorumThreshold {
				p.broadcast(
					context,
					&messages.ReliableProtocolMessage{
						Stage:       messages.ReliableProtocolMessage_VALIDATE,
						MessageData: msgData,
					})
				msgState.witnessStage = SentValidate
			}
		case messages.ReliableProtocolMessage_VALIDATE:
			if !msgState.ownWitnessSet[sender] || msgState.witnessesValidated[sender] {
				return
			}

			msgState.witnessesValidated[sender] = true
			msgState.validateMessagesStat[value]++

			if msgState.validateMessagesStat[value] >= config.WitnessThreshold {
				p.deliver(msgData)
			}
		}
	}
}

func (p *Process) Broadcast(context actor.SenderContext, value int64) {
	id := utils.PidToString(p.currPid)

	msgData := &messages.MessageData{
		Author:    id,
		SeqNumber: p.msgCounter,
		Value:     value,
	}
	msg := &messages.ReliableProtocolMessage{
		Stage: messages.ReliableProtocolMessage_INITIAL,
		MessageData: msgData,
	}

	msgState := p.initMessageState(msgData)
	p.broadcastToWitnesses(context, msg, msgState)

	p.msgCounter++
}
