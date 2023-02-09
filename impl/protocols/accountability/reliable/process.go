package reliable

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/protobuf/proto"
	"math"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/utils"
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

type RecoveryStage int32

const (
	InitialRecoveryStage RecoveryStage = iota
	SentRecover
	SentEcho
	SentReady
)

type messageState struct {
	echoFromProcesses     map[string]bool
	readyFromProcesses    map[string]bool
	readyFromWitnesses    map[string]bool
	validateFromWitnesses map[string]bool

	echoFromProcessesStat  map[protocols.ValueType]int
	readyFromProcessesStat map[protocols.ValueType]int
	readyFromWitnessesStat map[protocols.ValueType]int
	validatesStat          map[protocols.ValueType]int

	stage        Stage
	witnessStage WitnessStage

	ownWitnessSet map[string]bool
	potWitnessSet map[string]bool
}

func newMessageState() *messageState {
	ms := new(messageState)

	ms.echoFromProcesses = make(map[string]bool)
	ms.readyFromProcesses = make(map[string]bool)
	ms.readyFromWitnesses = make(map[string]bool)
	ms.validateFromWitnesses = make(map[string]bool)

	ms.echoFromProcessesStat = make(map[protocols.ValueType]int)
	ms.readyFromWitnessesStat = make(map[protocols.ValueType]int)
	ms.readyFromProcessesStat = make(map[protocols.ValueType]int)
	ms.validatesStat = make(map[protocols.ValueType]int)

	ms.stage = InitialStage
	ms.witnessStage = InitialWitnessStage

	return ms
}

type recoveryMessageState struct {
	receivedRecover map[string]bool
	receivedReply   map[string]bool
	receivedEcho    map[string]bool
	receivedReady   map[string]bool

	recoverMessagesStat map[protocols.ValueType]int
	replyMessagesStat   map[protocols.ValueType]int
	recoverReadyStat    map[protocols.ValueType]int
	echoMessagesStat    map[protocols.ValueType]int
	readyMessagesStat   map[protocols.ValueType]int

	stage RecoveryStage
}

func newRecoveryMessageState() *recoveryMessageState {
	ms := new(recoveryMessageState)

	ms.receivedRecover = make(map[string]bool)
	ms.receivedReply = make(map[string]bool)

	ms.recoverMessagesStat = make(map[protocols.ValueType]int)
	ms.replyMessagesStat = make(map[protocols.ValueType]int)

	ms.stage = InitialRecoveryStage

	return ms
}

type Process struct {
	currPid    *actor.PID
	pids       map[string]*actor.PID
	msgCounter int64

	acceptedMessages    map[string]map[int64]protocols.ValueType
	messagesLog         map[string]map[int64]*messageState
	lastSentPMessages   map[string]map[int64]*messages.ReliableProtocolMessage
	recoveryMessagesLog map[string]map[int64]*recoveryMessageState

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
	p.lastSentPMessages = make(map[string]map[int64]*messages.ReliableProtocolMessage)
	p.recoveryMessagesLog = make(map[string]map[int64]*recoveryMessageState)

	ids := make([]string, len(pids))
	for i, pid := range pids {
		id := utils.PidToString(pid)
		ids[i] = id
		p.pids[id] = pid
		p.acceptedMessages[id] = make(map[int64]protocols.ValueType)
		p.messagesLog[id] = make(map[int64]*messageState)
		p.lastSentPMessages[id] = make(map[int64]*messages.ReliableProtocolMessage)
		p.recoveryMessagesLog[id] = make(map[int64]*recoveryMessageState)
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

func (p *Process) initMessageState(context actor.SenderContext, msgData *messages.MessageData) *messageState {
	msgState := newMessageState()
	msgState.ownWitnessSet, msgState.potWitnessSet =
		p.wSelector.GetWitnessSet(msgData.Author, msgData.SeqNumber, p.historyHash)
	p.messagesLog[msgData.Author][msgData.SeqNumber] = msgState

	p.broadcastToWitnesses(
		context,
		&messages.ReliableProtocolMessage{
			Stage:       messages.ReliableProtocolMessage_NOTIFY,
			MessageData: msgData,
		},
		msgState)
	return msgState
}

func (p *Process) registerMessage(context actor.Context, msgData *messages.MessageData) *messageState {
	msgState := p.messagesLog[msgData.Author][msgData.SeqNumber]
	if msgState == nil {
		msgState = p.initMessageState(context, msgData)
		context.ReenterAfter(
			actor.NewFuture(context.ActorSystem(), config.RecoverySwitchTimeoutNs),
			func(res interface{}, err error) {
				if !p.accepted(msgData) {
					p.broadcastRecover(context, msgData)
				}
			})
	}
	return msgState
}

func (p *Process) getRecoveryMessageState(msgData *messages.MessageData) *recoveryMessageState {
	recoveryState := p.recoveryMessagesLog[msgData.Author][msgData.SeqNumber]
	if recoveryState == nil {
		recoveryState = newRecoveryMessageState()
		p.recoveryMessagesLog[msgData.Author][msgData.SeqNumber] = recoveryState
	}

	return recoveryState
}

func (p *Process) broadcast(context actor.SenderContext, message proto.Message) {
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

	msgData := message.GetMessageData()
	p.lastSentPMessages[msgData.Author][msgData.SeqNumber] = message
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

func (p *Process) broadcastRecover(
	context actor.SenderContext, msgData *messages.MessageData) {
	lastProcessMessage := p.lastSentPMessages[msgData.Author][msgData.SeqNumber]
	if lastProcessMessage == nil {
		fmt.Printf(
			"%s: Error, no process message for transaction with author: %s and seq number %d was sent\n",
			utils.PidToString(p.currPid),
			msgData.Author,
			msgData.SeqNumber)
	}

	p.broadcast(
		context,
		&messages.RecoveryMessage{
			RecoveryStage: messages.RecoveryMessage_RECOVER,
			Message:       lastProcessMessage,
		})

	recoveryState := p.getRecoveryMessageState(msgData)
	recoveryState.stage = SentRecover
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	reliableMessage *messages.ReliableProtocolMessage) {
	p.broadcast(
		context,
		&messages.RecoveryMessage{
			RecoveryStage: messages.RecoveryMessage_READY,
			Message:       reliableMessage,
		})

	recoveryState := p.getRecoveryMessageState(reliableMessage.GetMessageData())
	recoveryState.stage = SentReady
}

func (p *Process) isWitness(msgState *messageState) bool {
	return msgState.potWitnessSet[utils.PidToString(p.currPid)]
}

func (p *Process) accepted(msgData *messages.MessageData) bool {
	acceptedValue, accepted := p.acceptedMessages[msgData.Author][msgData.SeqNumber]
	if accepted {
		if acceptedValue != protocols.ValueType(msgData.Value) {
			fmt.Printf("%s: Detected a duplicated seq number attack\n", p.currPid.Id)
		}
	}
	return accepted
}

func isTaggedWithP(msg *messages.ReliableProtocolMessage) bool {
	return msg.Stage == messages.ReliableProtocolMessage_NOTIFY ||
		msg.Stage == messages.ReliableProtocolMessage_ECHO_FROM_PROCESS ||
		msg.Stage == messages.ReliableProtocolMessage_READY_FROM_PROCESS
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
		senderId := utils.PidToString(context.Sender())
		value := protocols.ValueType(msgData.Value)

		if p.accepted(msgData) {
			return
		}

		msgState := p.registerMessage(context, msgData)

		switch msg.Stage {
		case messages.ReliableProtocolMessage_NOTIFY:
			if !p.isWitness(msgState) || msgState.witnessStage >= SentEchoFromWitness {
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
			if !msgState.ownWitnessSet[senderId] || msgState.stage >= SentEchoFromProcess {
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
				msgState.echoFromProcesses[senderId] {
				return
			}

			msgState.echoFromProcesses[senderId] = true
			msgState.echoFromProcessesStat[value]++

			if msgState.echoFromProcessesStat[value] >= p.quorumThreshold {
				p.broadcastReadyFromWitness(context, msgData, msgState)
			}
		case messages.ReliableProtocolMessage_READY_FROM_WITNESS:
			if !msgState.ownWitnessSet[senderId] ||
				msgState.stage >= SentReadyFromProcess ||
				msgState.readyFromWitnesses[senderId] {
				return
			}

			msgState.readyFromWitnesses[senderId] = true
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
				msgState.readyFromProcesses[senderId] {
				return
			}

			msgState.readyFromProcesses[senderId] = true
			msgState.readyFromProcessesStat[value]++

			if msgState.witnessStage < SentReadyFromWitness &&
				msgState.readyFromProcessesStat[value] >= p.readyMessagesThreshold {
				p.broadcastReadyFromWitness(context, msgData, msgState)
			}

			if msgState.readyFromProcessesStat[value] >= p.quorumThreshold {
				p.broadcast(
					context,
					&messages.ReliableProtocolMessage{
						Stage:       messages.ReliableProtocolMessage_VALIDATE,
						MessageData: msgData,
					})
				msgState.witnessStage = SentValidate
			}
		case messages.ReliableProtocolMessage_VALIDATE:
			if !msgState.ownWitnessSet[senderId] || msgState.validateFromWitnesses[senderId] {
				return
			}

			msgState.validateFromWitnesses[senderId] = true
			msgState.validatesStat[value]++

			if msgState.validatesStat[value] >= config.WitnessThreshold {
				p.deliver(msgData)
			}
		}
	case messages.RecoveryMessage:
		msg := message.(*messages.RecoveryMessage)
		reliableMessage := msg.GetMessage()
		msgData := reliableMessage.GetMessageData()
		value := protocols.ValueType(msgData.Value)

		sender := context.Sender()
		senderId := utils.PidToString(sender)

		accepted := p.accepted(msgData)
		recoveryState := p.getRecoveryMessageState(msgData)

		switch msg.RecoveryStage {
		case messages.RecoveryMessage_RECOVER:
			if recoveryState.receivedRecover[senderId] || isTaggedWithP(reliableMessage) {
				return
			}

			recoveryState.receivedRecover[senderId] = true
			recoveryState.recoverMessagesStat[value]++
			if reliableMessage.Stage == messages.ReliableProtocolMessage_READY_FROM_PROCESS {
				recoveryState.recoverReadyStat[value]++
			}

			if accepted {
				context.RequestWithCustomSender(
					sender,
					messages.RecoveryMessage{
						RecoveryStage: messages.RecoveryMessage_REPLY,
						Message:       reliableMessage,
					},
					p.currPid)
			}
			recoverMessagesCnt := len(recoveryState.receivedRecover)

			if recoveryState.stage < SentRecover &&
				recoverMessagesCnt >= p.readyMessagesThreshold {
				p.broadcastRecover(context, msgData)
			}

			if recoveryState.stage < SentEcho &&
				(recoveryState.recoverReadyStat[value] >= p.readyMessagesThreshold ||
					recoverMessagesCnt >= p.quorumThreshold &&
						recoveryState.recoverMessagesStat[value] == recoverMessagesCnt) {
				p.broadcast(
					context,
					&messages.RecoveryMessage{
						RecoveryStage: messages.RecoveryMessage_ECHO,
						Message:       reliableMessage,
					})
				recoveryState.stage = SentEcho
			}
		case messages.RecoveryMessage_REPLY:
			if recoveryState.receivedReply[senderId] {
				return
			}

			recoveryState.receivedReply[senderId] = true
			recoveryState.replyMessagesStat[value]++

			if !accepted && recoveryState.replyMessagesStat[value] >= p.readyMessagesThreshold {
				p.deliver(msgData)
			}
		case messages.RecoveryMessage_ECHO:
			if recoveryState.receivedEcho[senderId] {
				return
			}

			recoveryState.receivedEcho[senderId] = true
			recoveryState.echoMessagesStat[value]++

			if recoveryState.stage < SentReady &&
				recoveryState.echoMessagesStat[value] >= p.quorumThreshold {
				p.broadcastReady(context, reliableMessage)
			}
		case messages.RecoveryMessage_READY:
			if recoveryState.receivedReady[senderId] {
				return
			}

			recoveryState.receivedReady[senderId] = true
			recoveryState.readyMessagesStat[value]++

			if recoveryState.stage < SentReady &&
				recoveryState.readyMessagesStat[value] >= p.readyMessagesThreshold {
				p.broadcastReady(context, reliableMessage)
			}

			if !accepted && recoveryState.readyMessagesStat[value] >= p.quorumThreshold {
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

	p.initMessageState(context, msgData)

	p.msgCounter++
}
