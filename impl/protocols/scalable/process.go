package scalable

import (
	"fmt"
	xrand "golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
	"math/rand"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"sync"
	"time"
)

type ProcessId int32

type messageState struct {
	receivedEcho  map[ProcessId]bool
	receivedReady map[ProcessId]map[int32]bool

	echoMessagesStat   map[int32]int
	readySampleStat    map[int32]int
	deliverySampleStat map[int32]int

	sentReadyMessages map[int32]bool

	gossipSample   map[ProcessId]int
	echoSample     map[ProcessId]int
	readySample    map[ProcessId]int
	deliverySample map[ProcessId]int

	echoSubscriptionSet  map[ProcessId]int
	readySubscriptionSet map[ProcessId]int

	gossipMessage *messages.ScalableProtocolMessage
	echoMessage   *messages.ScalableProtocolMessage

	sentReadyFromSieve bool

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)

	ms.receivedEcho = make(map[ProcessId]bool)
	ms.receivedReady = make(map[ProcessId]map[int32]bool)

	ms.echoMessagesStat = make(map[int32]int)
	ms.readySampleStat = make(map[int32]int)
	ms.deliverySampleStat = make(map[int32]int)

	ms.sentReadyMessages = make(map[int32]bool)

	ms.gossipSample = make(map[ProcessId]int)
	ms.echoSample = make(map[ProcessId]int)
	ms.readySample = make(map[ProcessId]int)
	ms.deliverySample = make(map[ProcessId]int)

	ms.echoSubscriptionSet = make(map[ProcessId]int)
	ms.readySubscriptionSet = make(map[ProcessId]int)

	ms.receivedMessagesCnt = 0

	return ms
}

type Process struct {
	processIndex int32
	n            int

	transactionCounter int32

	deliveredMessages map[ProcessId]map[int32]int32
	messagesLog       map[ProcessId]map[int32]*messageState
	logMutex map[ProcessId]*sync.RWMutex

	gossipSampleSize   int
	echoSampleSize     int
	echoThreshold      int
	readySampleSize    int
	readyThreshold     int
	deliverySampleSize int
	deliveryThreshold  int
	cleanUpTimeout     time.Duration

	logger *eventlogger.EventLogger
}

func (p *Process) getRandomPid(random *rand.Rand) ProcessId {
	return ProcessId(random.Int() % p.n)
}

func (p *Process) generateGossipSample() map[ProcessId]int {
	sample := make(map[ProcessId]int)

	seed := time.Now().UnixNano()

	poisson := distuv.Poisson{
		Lambda: float64(p.gossipSampleSize),
		Src:    xrand.NewSource(uint64(seed)),
	}

	gSize := int(poisson.Rand())
	if gSize > p.n {
		gSize = p.n
	}

	uniform := rand.New(rand.NewSource(seed))

	for len(sample) < gSize {
		sample[p.getRandomPid(uniform)]++
	}

	return sample
}

func (p *Process) sample(
	reliableContext *context.ReliableContext,
	stage messages.ScalableProtocolMessage_Stage,
	bInstance *messages.BroadcastInstance,
	value int32,
	size int,
) map[ProcessId]int {
	sample := make(map[ProcessId]int)
	random := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < size; i++ {
		sample[p.getRandomPid(random)]++
	}

	p.broadcastToSet(
		reliableContext,
		sample,
		bInstance,
		&messages.ScalableProtocolMessage{
			Stage: stage,
			Value: value,
		})

	return sample
}

func (p *Process) InitProcess(
	processIndex int32,
	actorPids []string,
	parameters *parameters.Parameters,
	logger *eventlogger.EventLogger,
) {
	p.processIndex = processIndex
	p.n = len(actorPids)

	p.transactionCounter = 0

	p.deliveredMessages = make(map[ProcessId]map[int32]int32)
	p.messagesLog = make(map[ProcessId]map[int32]*messageState)
	p.logMutex = make(map[ProcessId]*sync.RWMutex)

	for i := range actorPids {
		p.deliveredMessages[ProcessId(i)] = make(map[int32]int32)
		p.messagesLog[ProcessId(i)] = make(map[int32]*messageState)
		p.logMutex[ProcessId(i)] = &sync.RWMutex{}
	}

	p.gossipSampleSize = parameters.GossipSampleSize
	p.echoSampleSize = parameters.EchoSampleSize
	p.echoThreshold = parameters.EchoThreshold
	p.readySampleSize = parameters.ReadySampleSize
	p.readyThreshold = parameters.ReadyThreshold
	p.deliverySampleSize = parameters.DeliverySampleSize
	p.deliveryThreshold = parameters.DeliveryThreshold
	p.cleanUpTimeout = time.Duration(parameters.CleanUpTimeout)

	p.logger = logger
}

func (p *Process) initMessageState(
	reliableContext *context.ReliableContext,
	bInstance *messages.BroadcastInstance,
	value int32,
) *messageState {
	author := ProcessId(bInstance.Author)
	msgState := newMessageState()

	for i := 0; i < p.n; i++ {
		msgState.receivedReady[ProcessId(i)] = make(map[int32]bool)
	}

	msgState.gossipSample = p.generateGossipSample()
	p.broadcastToSet(
		reliableContext,
		msgState.gossipSample,
		bInstance,
		&messages.ScalableProtocolMessage{
			Stage: messages.ScalableProtocolMessage_GOSSIP_SUBSCRIBE,
			Value: value,
		})

	msgState.echoSample =
		p.sample(
			reliableContext,
			messages.ScalableProtocolMessage_ECHO_SUBSCRIBE,
			bInstance,
			value,
			p.echoSampleSize)
	msgState.readySample =
		p.sample(
			reliableContext,
			messages.ScalableProtocolMessage_READY_SUBSCRIBE,
			bInstance,
			value,
			p.readySampleSize)
	msgState.deliverySample =
		p.sample(
			reliableContext,
			messages.ScalableProtocolMessage_READY_SUBSCRIBE,
			bInstance,
			value,
			p.deliverySampleSize)

	p.messagesLog[author][bInstance.SeqNumber] = msgState

	return msgState
}

func (p *Process) sendMessage(
	reliableContext *context.ReliableContext,
	to ProcessId,
	bInstance *messages.BroadcastInstance,
	message *messages.ScalableProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_ScalableProtocolMessage{
			ScalableProtocolMessage: message.Copy(),
		},
	}

	msg := reliableContext.MakeNewMessage()
	msg.Content = &messages.Message_BroadcastInstanceMessage{
		BroadcastInstanceMessage: bMessage,
	}

	reliableContext.Send(int32(to), msg)
}

func (p *Process) broadcastToSet(
	reliableContext *context.ReliableContext,
	set map[ProcessId]int,
	bInstance *messages.BroadcastInstance,
	msg *messages.ScalableProtocolMessage,
) {
	for pid := range set {
		p.sendMessage(reliableContext, pid, bInstance, msg)
	}
}

func (p *Process) broadcastGossip(
	reliableContext *context.ReliableContext,
	msgState *messageState,
	bInstance *messages.BroadcastInstance,
	value int32,
) {
	msgState.gossipMessage =
		&messages.ScalableProtocolMessage{
			Stage: messages.ScalableProtocolMessage_GOSSIP,
			Value: value,
		}
	p.broadcastToSet(
		reliableContext,
		msgState.gossipSample,
		bInstance,
		msgState.gossipMessage)
}

func (p *Process) broadcastReady(
	reliableContext *context.ReliableContext,
	msgState *messageState,
	bInstance *messages.BroadcastInstance,
	value int32,
) {
	msgState.sentReadyMessages[value] = true

	p.broadcastToSet(
		reliableContext,
		msgState.readySubscriptionSet,
		bInstance,
		&messages.ScalableProtocolMessage{
			Stage: messages.ScalableProtocolMessage_READY,
			Value: value,
		})
}

func (p *Process) delivered(bInstance *messages.BroadcastInstance, value int32) bool {
	deliveredValue, delivered :=
		p.deliveredMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber]

	if delivered && deliveredValue != value {
		p.logger.OnAttack(bInstance, value, deliveredValue)
	}

	return delivered
}

func (p *Process) deliver(
	bInstance *messages.BroadcastInstance,
	value int32,
) {
	author := ProcessId(bInstance.Author)
	p.deliveredMessages[author][bInstance.SeqNumber] = value
	messagesReceived := p.messagesLog[author][bInstance.SeqNumber].receivedMessagesCnt

	p.logger.OnDeliver(bInstance, value, messagesReceived)

	go func() {
		time.Sleep(p.cleanUpTimeout)

		p.logMutex[author].Lock()
		delete(p.messagesLog[author], bInstance.SeqNumber)
		p.logMutex[author].Unlock()
	}()
}

func (p *Process) maybeSendReadyFromSieve(
	reliableContext *context.ReliableContext,
	msgState *messageState,
	bInstance *messages.BroadcastInstance,
) {
	if !msgState.sentReadyFromSieve && msgState.echoMessage != nil {
		echoValue := msgState.echoMessage.Value
		if msgState.echoMessagesStat[echoValue] >= p.echoThreshold {
			p.broadcastReady(reliableContext, msgState, bInstance, echoValue)
			msgState.sentReadyFromSieve = true
		}
	}
}

func (p *Process) processProtocolMessage(
	reliableContext *context.ReliableContext,
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	message *messages.ScalableProtocolMessage,
) {
	value := message.Value
	author := ProcessId(bInstance.Author)

	p.logMutex[author].Lock()
	defer p.logMutex[author].Unlock()

	msgState := p.messagesLog[author][bInstance.SeqNumber]

	if msgState == nil {
		if p.delivered(bInstance, value) {
			return
		} else {
			msgState = p.initMessageState(reliableContext, bInstance, value)
		}
	}

	msgState.receivedMessagesCnt++

	switch message.Stage {
	case messages.ScalableProtocolMessage_GOSSIP_SUBSCRIBE:
		msgState.gossipSample[senderId] = 1
		if msgState.gossipMessage != nil {
			p.sendMessage(
				reliableContext,
				senderId,
				bInstance,
				msgState.gossipMessage)
		}
	case messages.ScalableProtocolMessage_GOSSIP:
		if msgState.gossipMessage == nil {
			p.broadcastGossip(reliableContext, msgState, bInstance, value)
		}
		if msgState.echoMessage == nil {
			msgState.echoMessage = &messages.ScalableProtocolMessage{
				Stage: messages.ScalableProtocolMessage_ECHO,
				Value: value,
			}
			p.broadcastToSet(
				reliableContext,
				msgState.echoSubscriptionSet,
				bInstance,
				msgState.echoMessage)
			p.maybeSendReadyFromSieve(reliableContext, msgState, bInstance)
		}
	case messages.ScalableProtocolMessage_ECHO_SUBSCRIBE:
		if msgState.echoSubscriptionSet[senderId] > 0 {
			return
		}

		msgState.echoSubscriptionSet[senderId] = 1
		if msgState.echoMessage != nil {
			p.sendMessage(reliableContext, senderId, bInstance, msgState.echoMessage)
		}
	case messages.ScalableProtocolMessage_ECHO:
		if msgState.echoSample[senderId] == 0 || msgState.receivedEcho[senderId] {
			return
		}

		msgState.receivedEcho[senderId] = true
		msgState.echoMessagesStat[value] += msgState.echoSample[senderId]

		p.maybeSendReadyFromSieve(reliableContext, msgState, bInstance)
	case messages.ScalableProtocolMessage_READY_SUBSCRIBE:
		if msgState.readySubscriptionSet[senderId] > 0 {
			return
		}

		msgState.readySubscriptionSet[senderId] = 1

		for val := range msgState.sentReadyMessages {
			p.sendMessage(
				reliableContext,
				senderId,
				bInstance,
				&messages.ScalableProtocolMessage{
					Stage: messages.ScalableProtocolMessage_READY,
					Value: val,
				},
			)
		}
	case messages.ScalableProtocolMessage_READY:
		if msgState.readySample[senderId] > 0 {
			msgState.readySampleStat[value] += msgState.readySample[senderId]

			if !msgState.sentReadyMessages[value] &&
				msgState.readySampleStat[value] >= p.readyThreshold {
				p.broadcastReady(reliableContext, msgState, bInstance, value)
			}
		}

		if msgState.deliverySample[senderId] > 0 {
			msgState.deliverySampleStat[value] += msgState.deliverySample[senderId]

			if !p.delivered(bInstance, value) &&
				msgState.deliverySampleStat[value] >= p.deliveryThreshold {
				p.deliver(bInstance, value)
			}
		}
	}
}

func (p *Process) HandleMessage(
	reliableContext *context.ReliableContext,
	sender int32,
	broadcastInstanceMessage *messages.BroadcastInstanceMessage,
) {
	bInstance := broadcastInstanceMessage.BroadcastInstance

	switch protocolMessage := broadcastInstanceMessage.Message.(type) {
	case *messages.BroadcastInstanceMessage_ScalableProtocolMessage:
		p.processProtocolMessage(
			reliableContext,
			ProcessId(sender),
			bInstance,
			protocolMessage.ScalableProtocolMessage,
		)
	default:
		p.logger.Fatal(fmt.Sprintf("Invalid protocol message type %t", protocolMessage))
	}
}

func (p *Process) Broadcast(
	reliableContext *context.ReliableContext,
	value int32,
) {
	author := p.processIndex
	p.logMutex[ProcessId(author)].Lock()
	defer p.logMutex[ProcessId(author)].Unlock()

	broadcastInstance := &messages.BroadcastInstance{
		Author:    author,
		SeqNumber: p.transactionCounter,
	}

	msgState := p.initMessageState(reliableContext, broadcastInstance, value)
	p.broadcastGossip(reliableContext, msgState, broadcastInstance, value)

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
