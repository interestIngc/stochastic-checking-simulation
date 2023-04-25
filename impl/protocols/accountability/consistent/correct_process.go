package consistent

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"math"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/utils"
)

type ProcessId int64

type messageState struct {
	receivedEcho map[ProcessId]bool
	echoCount    map[int64]int
	witnessSet   map[string]bool

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)

	ms.receivedEcho = make(map[ProcessId]bool)
	ms.echoCount = make(map[int64]int)

	ms.receivedMessagesCnt = 0

	return ms
}

type CorrectProcess struct {
	processIndex int64
	actorPids    map[string]*actor.PID
	pids         []string

	transactionCounter int64
	messageCounter     int64

	deliveredMessages        map[ProcessId]map[int64]int64
	deliveredMessagesHistory []string
	messagesLog              map[ProcessId]map[int64]*messageState

	witnessThreshold int

	wSelector   *hashing.WitnessesSelector
	historyHash *hashing.HistoryHash

	logger             *eventlogger.EventLogger
	transactionManager *protocols.TransactionManager

	mainServer *actor.PID
}

func (p *CorrectProcess) InitProcess(
	processIndex int64,
	actorPids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger,
	transactionManager *protocols.TransactionManager,
	mainServer *actor.PID,
) {
	p.processIndex = processIndex
	p.actorPids = make(map[string]*actor.PID)
	p.pids = make([]string, len(actorPids))

	p.transactionCounter = 0
	p.messageCounter = 0

	p.deliveredMessages = make(map[ProcessId]map[int64]int64)
	p.messagesLog = make(map[ProcessId]map[int64]*messageState)

	p.witnessThreshold = parameters.WitnessThreshold

	for i, actorPid := range actorPids {
		pid := utils.MakeCustomPid(actorPid)
		p.pids[i] = pid
		p.actorPids[pid] = actorPid
		p.deliveredMessages[ProcessId(i)] = make(map[int64]int64)
		p.messagesLog[ProcessId(i)] = make(map[int64]*messageState)
	}

	var hasher hashing.Hasher
	if parameters.NodeIdSize == 256 {
		hasher = hashing.HashSHA256{}
	} else {
		hasher = hashing.HashSHA512{}
	}

	p.wSelector = &hashing.WitnessesSelector{
		Hasher:               hasher,
		MinPotWitnessSetSize: parameters.MinPotWitnessSetSize,
		MinOwnWitnessSetSize: parameters.MinOwnWitnessSetSize,
		PotWitnessSetRadius:  parameters.PotWitnessSetRadius,
		OwnWitnessSetRadius:  parameters.OwnWitnessSetRadius,
	}
	binCapacity := uint(math.Pow(2, float64(parameters.NodeIdSize/parameters.NumberOfBins)))
	p.historyHash = hashing.NewHistoryHash(uint(parameters.NumberOfBins), binCapacity, hasher)

	p.logger = eventlogger.InitEventLogger(p.processIndex, logger)
	p.transactionManager = transactionManager

	p.mainServer = mainServer
}

func (p *CorrectProcess) initMessageState(
	bInstance *messages.BroadcastInstance,
) *messageState {
	msgState := newMessageState()
	p.messagesLog[ProcessId(bInstance.Author)][bInstance.SeqNumber] = msgState

	p.logger.OnHistoryUsedInWitnessSetSelection(
		bInstance,
		p.historyHash,
		p.deliveredMessagesHistory,
	)

	msgState.witnessSet, _ =
		p.wSelector.GetWitnessSet(p.pids, bInstance.Author, bInstance.SeqNumber, p.historyHash)

	p.logger.OnWitnessSetSelected("own", bInstance, msgState.witnessSet)

	return msgState
}

func (p *CorrectProcess) sendMessage(
	context actor.SenderContext,
	to *actor.PID,
	bInstance *messages.BroadcastInstance,
	message *messages.ConsistentProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Sender:            p.processIndex,
		Message: &messages.BroadcastInstanceMessage_ConsistentProtocolMessage{
			ConsistentProtocolMessage: message.Copy(),
		},
		Stamp: p.messageCounter,
	}

	context.Send(to, bMessage)

	p.logger.OnMessageSent(p.messageCounter)
	p.messageCounter++
}

func (p *CorrectProcess) broadcast(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	message *messages.ConsistentProtocolMessage) {
	for _, pid := range p.actorPids {
		p.sendMessage(context, pid, bInstance, message)
	}
}

func (p *CorrectProcess) deliver(bInstance *messages.BroadcastInstance, value int64) {
	author := ProcessId(bInstance.Author)

	p.deliveredMessages[author][bInstance.SeqNumber] = value
	p.deliveredMessagesHistory = append(p.deliveredMessagesHistory, bInstance.ToString())
	p.historyHash.Insert(
		utils.TransactionToBytes(p.pids[bInstance.Author], bInstance.SeqNumber))

	messagesReceived :=
		p.messagesLog[author][bInstance.SeqNumber].receivedMessagesCnt

	delete(p.messagesLog[author], bInstance.SeqNumber)
	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *CorrectProcess) verify(
	context actor.SenderContext,
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	value int64,
) bool {
	author := ProcessId(bInstance.Author)
	msgState := p.messagesLog[author][bInstance.SeqNumber]

	deliveredValue, delivered :=
		p.deliveredMessages[author][bInstance.SeqNumber]
	if delivered {
		if deliveredValue != value {
			p.logger.OnAttack(bInstance, value, deliveredValue)
			return false
		}
	} else if msgState != nil {
		msgState.receivedMessagesCnt++
		if msgState.witnessSet[p.pids[senderId]] && !msgState.receivedEcho[senderId] {
			msgState.receivedEcho[senderId] = true
			msgState.echoCount[value]++
			if msgState.echoCount[value] >= p.witnessThreshold {
				p.deliver(bInstance, value)
			}
		}
	} else {
		msgState = p.initMessageState(bInstance)
		msgState.receivedMessagesCnt++

		message := &messages.ConsistentProtocolMessage{
			Stage: messages.ConsistentProtocolMessage_VERIFY,
			Value: value,
		}
		for pid := range msgState.witnessSet {
			p.sendMessage(context, p.actorPids[pid], bInstance, message)
		}
	}
	return true
}

func (p *CorrectProcess) Receive(context actor.Context) {
	switch message := context.Message().(type) {
	case *actor.Started:
		p.logger.OnStart()
		context.Send(
			p.mainServer,
			&messages.Started{Sender: p.processIndex},
		)
	case *actor.Stop:
		p.logger.OnStop()
	case *messages.Simulate:
		p.transactionManager.Simulate(context, p)
	case *messages.BroadcastInstanceMessage:
		bInstance := message.BroadcastInstance

		p.logger.OnMessageReceived(message.Sender, message.Stamp)

		switch protocolMessage := message.Message.(type) {
		case *messages.BroadcastInstanceMessage_ConsistentProtocolMessage:
			consistentMessage := protocolMessage.ConsistentProtocolMessage

			doBroadcast := p.verify(context, ProcessId(message.Sender), bInstance, consistentMessage.Value)

			if consistentMessage.Stage == messages.ConsistentProtocolMessage_VERIFY && doBroadcast {
				p.broadcast(
					context,
					bInstance,
					&messages.ConsistentProtocolMessage{
						Stage: messages.ConsistentProtocolMessage_ECHO,
						Value: consistentMessage.Value,
					})
			}
		default:
			p.logger.Fatal(fmt.Sprintf("Invalid protocol message type %t", protocolMessage))
		}
	}
}

func (p *CorrectProcess) Broadcast(context actor.SenderContext, value int64) {
	broadcastInstance := &messages.BroadcastInstance{
		Author:    p.processIndex,
		SeqNumber: p.transactionCounter,
	}

	p.verify(context, ProcessId(p.processIndex), broadcastInstance, value)

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
