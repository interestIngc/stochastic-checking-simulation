package consistent

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"math"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/manager"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/utils"
)

type messageState struct {
	receivedEcho map[string]bool
	echoCount    map[protocols.ValueType]int
	witnessSet   map[string]bool

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)

	ms.receivedEcho = make(map[string]bool)
	ms.echoCount = make(map[protocols.ValueType]int)

	ms.receivedMessagesCnt = 0

	return ms
}

type CorrectProcess struct {
	actorPid  *actor.PID
	pid       string
	actorPids map[string]*actor.PID

	msgCounter int64

	acceptedMessages map[string]map[int64]protocols.ValueType
	messagesLog      map[string]map[int64]*messageState

	witnessThreshold int

	wSelector   *hashing.WitnessesSelector
	historyHash *hashing.HistoryHash

	transactionManager *manager.TransactionManager
	logger             *eventlogger.EventLogger
}

func (p *CorrectProcess) InitProcess(
	actorPid *actor.PID,
	actorPids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger) {
	p.actorPid = actorPid
	p.pid = utils.MakeCustomPid(actorPid)
	p.actorPids = make(map[string]*actor.PID)

	p.msgCounter = 0

	p.acceptedMessages = make(map[string]map[int64]protocols.ValueType)
	p.messagesLog = make(map[string]map[int64]*messageState)

	p.witnessThreshold = parameters.WitnessThreshold

	pids := make([]string, len(actorPids))
	for i, currActorPid := range actorPids {
		pid := utils.MakeCustomPid(currActorPid)
		pids[i] = pid
		p.actorPids[pid] = currActorPid
		p.acceptedMessages[pid] = make(map[int64]protocols.ValueType)
		p.messagesLog[pid] = make(map[int64]*messageState)
	}

	var hasher hashing.Hasher
	if parameters.NodeIdSize == 256 {
		hasher = hashing.HashSHA256{}
	} else {
		hasher = hashing.HashSHA512{}
	}

	p.wSelector = &hashing.WitnessesSelector{
		NodeIds:              pids,
		Hasher:               hasher,
		MinPotWitnessSetSize: parameters.MinPotWitnessSetSize,
		MinOwnWitnessSetSize: parameters.MinOwnWitnessSetSize,
		PotWitnessSetRadius:  parameters.PotWitnessSetRadius,
		OwnWitnessSetRadius:  parameters.OwnWitnessSetRadius,
	}
	binCapacity := uint(math.Pow(2, float64(parameters.NodeIdSize/parameters.NumberOfBins)))
	p.historyHash = hashing.NewHistoryHash(uint(parameters.NumberOfBins), binCapacity, hasher)

	p.logger = eventlogger.InitEventLogger(p.pid, logger)
}

func (p *CorrectProcess) initMessageState(msgData *messages.MessageData) *messageState {
	msgState := newMessageState()
	msgState.witnessSet, _ = p.wSelector.GetWitnessSet(msgData.Author, msgData.SeqNumber, p.historyHash)

	p.messagesLog[msgData.Author][msgData.SeqNumber] = msgState

	return msgState
}

func (p *CorrectProcess) sendMessage(
	context actor.SenderContext,
	to *actor.PID,
	message *messages.ConsistentProtocolMessage) {
	message.Timestamp = utils.GetNow()
	context.RequestWithCustomSender(to, message, p.actorPid)
}

func (p *CorrectProcess) broadcast(context actor.SenderContext, message *messages.ConsistentProtocolMessage) {
	for _, pid := range p.actorPids {
		p.sendMessage(context, pid, message)
	}
}

func (p *CorrectProcess) deliver(msgData *messages.MessageData) {
	p.acceptedMessages[msgData.Author][msgData.SeqNumber] = protocols.ValueType(msgData.Value)
	p.historyHash.Insert(
		utils.TransactionToBytes(msgData.Author, msgData.SeqNumber))
	messagesReceived := p.messagesLog[msgData.Author][msgData.SeqNumber].receivedMessagesCnt
	delete(p.messagesLog[msgData.Author], msgData.SeqNumber)

	p.logger.LogAccept(msgData, messagesReceived)
	p.logger.LogHistoryHash(p.historyHash)
}

func (p *CorrectProcess) verify(
	context actor.SenderContext, senderPid string, msgData *messages.MessageData) bool {
	value := protocols.ValueType(msgData.Value)
	msgState := p.messagesLog[msgData.Author][msgData.SeqNumber]

	acceptedValue, accepted := p.acceptedMessages[msgData.Author][msgData.SeqNumber]
	if accepted {
		if acceptedValue != value {
			p.logger.LogAttack()
			return false
		}
	} else if msgState != nil {
		msgState.receivedMessagesCnt++
		if msgState.witnessSet[senderPid] && !msgState.receivedEcho[senderPid] {
			msgState.receivedEcho[senderPid] = true
			msgState.echoCount[value]++
			if msgState.echoCount[value] >= p.witnessThreshold {
				p.deliver(msgData)
			}
		}
	} else {
		msgState = p.initMessageState(msgData)
		msgState.receivedMessagesCnt++

		message := &messages.ConsistentProtocolMessage{
			Stage:       messages.ConsistentProtocolMessage_VERIFY,
			MessageData: msgData,
		}
		for pid := range msgState.witnessSet {
			p.sendMessage(context, p.actorPids[pid], message)
		}
	}
	return true
}

func (p *CorrectProcess) Receive(context actor.Context) {
	message := context.Message()
	switch message.(type) {
	case *messages.Simulate:
		msg := message.(*messages.Simulate)

		p.logger.LogMessageLatency(utils.MakeCustomPid(context.Sender()), msg.Timestamp)

		p.transactionManager = &manager.TransactionManager{
			TransactionsToSendOut: msg.Transactions,
		}
		p.transactionManager.SendOutTransaction(context, p)
	case *messages.ConsistentProtocolMessage:
		msg := message.(*messages.ConsistentProtocolMessage)
		msgData := msg.GetMessageData()
		senderPid := utils.MakeCustomPid(context.Sender())

		p.logger.LogMessageLatency(senderPid, msg.Timestamp)

		doBroadcast := p.verify(context, senderPid, msgData)

		if msg.Stage == messages.ConsistentProtocolMessage_VERIFY && doBroadcast {
			p.broadcast(
				context,
				&messages.ConsistentProtocolMessage{
					Stage:       messages.ConsistentProtocolMessage_ECHO,
					MessageData: msgData,
				})
		}
	}
}

func (p *CorrectProcess) Broadcast(context actor.SenderContext, value int64) {
	p.verify(
		context,
		p.pid,
		&messages.MessageData{
			Author:    p.pid,
			SeqNumber: p.msgCounter,
			Value:     value,
		})

	p.msgCounter++
}
