package scalable

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	xrand "golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
	"log"
	"math/rand"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/utils"
	"time"
)

type messageState struct {
	receivedEcho  map[string]bool
	receivedReady map[string]map[int64]bool

	echoMessagesStat   map[int64]int
	readySampleStat    map[int64]int
	deliverySampleStat map[int64]int

	sentReadyMessages map[int64]bool

	gossipSample   map[string]bool
	echoSample     map[string]bool
	readySample    map[string]bool
	deliverySample map[string]bool

	echoSubscriptionSet  map[string]bool
	readySubscriptionSet map[string]bool

	gossipMessage *messages.ScalableProtocolMessage
	echoMessage   *messages.ScalableProtocolMessage

	sentReadyFromSieve bool

	receivedMessagesCnt int
}

func newMessageState() *messageState {
	ms := new(messageState)

	ms.receivedEcho = make(map[string]bool)
	ms.receivedReady = make(map[string]map[int64]bool)

	ms.echoMessagesStat = make(map[int64]int)
	ms.readySampleStat = make(map[int64]int)
	ms.deliverySampleStat = make(map[int64]int)

	ms.sentReadyMessages = make(map[int64]bool)

	ms.gossipSample = make(map[string]bool)
	ms.echoSample = make(map[string]bool)
	ms.readySample = make(map[string]bool)
	ms.deliverySample = make(map[string]bool)

	ms.echoSubscriptionSet = make(map[string]bool)
	ms.readySubscriptionSet = make(map[string]bool)

	ms.receivedMessagesCnt = 0

	return ms
}

type Process struct {
	actorPid  *actor.PID
	pid       string
	actorPids map[string]*actor.PID
	pids      []string

	transactionCounter int64
	messageCounter     int64

	deliveredMessages map[string]map[int64]int64
	messagesLog       map[string]map[int64]*messageState

	gossipSampleSize   int
	echoSampleSize     int
	echoThreshold      int
	readySampleSize    int
	readyThreshold     int
	deliverySampleSize int
	deliveryThreshold  int

	logger             *eventlogger.EventLogger
	transactionManager *protocols.TransactionManager

	mainServer *actor.PID
}

func (p *Process) getRandomPid(random *rand.Rand) string {
	return p.pids[random.Int()%len(p.pids)]
}

func (p *Process) generateGossipSample() map[string]bool {
	sample := make(map[string]bool)

	seed := time.Now().UnixNano()

	poisson := distuv.Poisson{
		Lambda: float64(p.gossipSampleSize),
		Src:    xrand.NewSource(uint64(seed)),
	}

	gSize := int(poisson.Rand())
	if gSize > len(p.pids) {
		gSize = len(p.pids)
	}

	uniform := rand.New(rand.NewSource(seed))

	for len(sample) < gSize {
		sample[p.getRandomPid(uniform)] = true
	}

	return sample
}

func (p *Process) sample(
	context actor.SenderContext,
	stage messages.ScalableProtocolMessage_Stage,
	bInstance *messages.BroadcastInstance,
	value int64,
	size int,
) map[string]bool {
	sample := make(map[string]bool)
	random := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < size; i++ {
		sample[p.getRandomPid(random)] = true
	}

	p.broadcastToSet(
		context,
		sample,
		bInstance,
		&messages.ScalableProtocolMessage{
			Stage: stage,
			Value: value,
		})

	return sample
}

func (p *Process) InitProcess(
	actorPid *actor.PID,
	actorPids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger,
	transactionManager *protocols.TransactionManager,
	mainServer *actor.PID,
) {
	p.actorPid = actorPid
	p.pid = utils.MakeCustomPid(actorPid)
	p.pids = make([]string, len(actorPids))
	p.actorPids = make(map[string]*actor.PID)

	p.transactionCounter = 0
	p.messageCounter = 0

	p.deliveredMessages = make(map[string]map[int64]int64)
	p.messagesLog = make(map[string]map[int64]*messageState)

	for i, currActorPid := range actorPids {
		pid := utils.MakeCustomPid(currActorPid)
		p.pids[i] = pid
		p.actorPids[pid] = currActorPid
		p.deliveredMessages[pid] = make(map[int64]int64)
		p.messagesLog[pid] = make(map[int64]*messageState)
	}

	p.gossipSampleSize = parameters.GossipSampleSize
	p.echoSampleSize = parameters.EchoSampleSize
	p.echoThreshold = parameters.EchoThreshold
	p.readySampleSize = parameters.ReadySampleSize
	p.readyThreshold = parameters.ReadyThreshold
	p.deliverySampleSize = parameters.DeliverySampleSize
	p.deliveryThreshold = parameters.DeliveryThreshold

	p.logger = eventlogger.InitEventLogger(p.pid, logger)
	p.transactionManager = transactionManager

	p.mainServer = mainServer
}

func (p *Process) initMessageState(
	context actor.SenderContext,
	bInstance *messages.BroadcastInstance,
	value int64,
) *messageState {
	msgState := p.messagesLog[bInstance.Author][bInstance.SeqNumber]

	if msgState == nil {
		msgState = newMessageState()

		for _, pid := range p.pids {
			msgState.receivedReady[pid] = make(map[int64]bool)
		}

		msgState.gossipSample = p.generateGossipSample()
		p.broadcastToSet(
			context,
			msgState.gossipSample,
			bInstance,
			&messages.ScalableProtocolMessage{
				Stage: messages.ScalableProtocolMessage_GOSSIP_SUBSCRIBE,
				Value: value,
			})

		msgState.echoSample =
			p.sample(
				context,
				messages.ScalableProtocolMessage_ECHO_SUBSCRIBE,
				bInstance,
				value,
				p.echoSampleSize)
		msgState.readySample =
			p.sample(
				context,
				messages.ScalableProtocolMessage_READY_SUBSCRIBE,
				bInstance,
				value,
				p.readySampleSize)
		msgState.deliverySample =
			p.sample(
				context,
				messages.ScalableProtocolMessage_READY_SUBSCRIBE,
				bInstance,
				value,
				p.deliverySampleSize)

		p.messagesLog[bInstance.Author][bInstance.SeqNumber] = msgState
	}

	return msgState
}

func (p *Process) sendMessage(
	context actor.SenderContext,
	to *actor.PID,
	bInstance *messages.BroadcastInstance,
	message *messages.ScalableProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_ScalableProtocolMessage{
			ScalableProtocolMessage: message.Copy(),
		},
		Stamp: p.messageCounter,
	}

	context.RequestWithCustomSender(to, bMessage, p.actorPid)

	p.logger.OnMessageSent(p.messageCounter)
	p.messageCounter++
}

func (p *Process) broadcastToSet(
	context actor.SenderContext,
	set map[string]bool,
	bInstance *messages.BroadcastInstance,
	msg *messages.ScalableProtocolMessage,
) {
	for pid := range set {
		p.sendMessage(context, p.actorPids[pid], bInstance, msg)
	}
}

func (p *Process) broadcastGossip(
	context actor.SenderContext,
	msgState *messageState,
	bInstance *messages.BroadcastInstance,
	value int64,
) {
	msgState.gossipMessage =
		&messages.ScalableProtocolMessage{
			Stage: messages.ScalableProtocolMessage_GOSSIP,
			Value: value,
		}
	p.broadcastToSet(
		context,
		msgState.gossipSample,
		bInstance,
		msgState.gossipMessage)
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	msgState *messageState,
	bInstance *messages.BroadcastInstance,
	value int64,
) {
	msgState.sentReadyMessages[value] = true

	p.broadcastToSet(
		context,
		msgState.readySubscriptionSet,
		bInstance,
		&messages.ScalableProtocolMessage{
			Stage: messages.ScalableProtocolMessage_READY,
			Value: value,
		})
}

func (p *Process) delivered(bInstance *messages.BroadcastInstance, value int64) bool {
	deliveredValue, delivered :=
		p.deliveredMessages[bInstance.Author][bInstance.SeqNumber]

	if delivered && deliveredValue != value {
		p.logger.OnAttack(bInstance, value, deliveredValue)
	}

	return delivered
}

func (p *Process) deliver(bInstance *messages.BroadcastInstance, value int64) {
	p.deliveredMessages[bInstance.Author][bInstance.SeqNumber] = value
	messagesReceived :=
		p.messagesLog[bInstance.Author][bInstance.SeqNumber].receivedMessagesCnt

	p.logger.OnDeliver(bInstance, value, messagesReceived)
}

func (p *Process) maybeSendReadyFromSieve(
	context actor.SenderContext,
	msgState *messageState,
	bInstance *messages.BroadcastInstance,
	value int64,
) {
	if !msgState.sentReadyFromSieve &&
		msgState.echoMessage != nil &&
		value == msgState.echoMessage.Value &&
		msgState.echoMessagesStat[value] >= p.echoThreshold {
		p.broadcastReady(context, msgState, bInstance, value)
		msgState.sentReadyFromSieve = true
	}
}

func (p *Process) processProtocolMessage(
	context actor.Context,
	senderPid string,
	bInstance *messages.BroadcastInstance,
	message *messages.ScalableProtocolMessage,
) {
	value := message.Value

	msgState := p.initMessageState(context, bInstance, value)
	msgState.receivedMessagesCnt++

	switch message.Stage {
	case messages.ScalableProtocolMessage_GOSSIP_SUBSCRIBE:
		if msgState.gossipSample[senderPid] {
			return
		}

		msgState.gossipSample[senderPid] = true
		if msgState.gossipMessage != nil {
			p.sendMessage(context, p.actorPids[senderPid], bInstance, msgState.gossipMessage)
		}
	case messages.ScalableProtocolMessage_GOSSIP:
		if msgState.gossipMessage == nil {
			p.broadcastGossip(context, msgState, bInstance, value)
		}
		if msgState.echoMessage == nil {
			msgState.echoMessage = &messages.ScalableProtocolMessage{
				Stage: messages.ScalableProtocolMessage_ECHO,
				Value: value,
			}
			p.broadcastToSet(
				context,
				msgState.echoSubscriptionSet,
				bInstance,
				msgState.echoMessage)
			p.maybeSendReadyFromSieve(context, msgState, bInstance, value)
		}
	case messages.ScalableProtocolMessage_ECHO_SUBSCRIBE:
		if msgState.echoSubscriptionSet[senderPid] {
			return
		}

		msgState.echoSubscriptionSet[senderPid] = true
		if msgState.echoMessage != nil {
			p.sendMessage(context, p.actorPids[senderPid], bInstance, msgState.echoMessage)
		}
	case messages.ScalableProtocolMessage_ECHO:
		if !msgState.echoSample[senderPid] || msgState.receivedEcho[senderPid] {
			return
		}

		msgState.receivedEcho[senderPid] = true
		msgState.echoMessagesStat[value]++

		p.maybeSendReadyFromSieve(context, msgState, bInstance, value)
	case messages.ScalableProtocolMessage_READY_SUBSCRIBE:
		if msgState.readySubscriptionSet[senderPid] {
			return
		}

		msgState.readySubscriptionSet[senderPid] = true

		for val := range msgState.sentReadyMessages {
			p.sendMessage(
				context,
				p.actorPids[senderPid],
				bInstance,
				&messages.ScalableProtocolMessage{
					Stage: messages.ScalableProtocolMessage_READY,
					Value: val,
				},
			)
		}
	case messages.ScalableProtocolMessage_READY:
		if msgState.receivedReady[senderPid][value] {
			return
		}

		msgState.receivedReady[senderPid][value] = true

		if msgState.readySample[senderPid] {
			msgState.readySampleStat[value]++

			if !msgState.sentReadyMessages[value] &&
				msgState.readySampleStat[value] >= p.readyThreshold {
				p.broadcastReady(context, msgState, bInstance, value)
			}
		}

		if msgState.deliverySample[senderPid] {
			msgState.deliverySampleStat[value]++

			if !p.delivered(bInstance, value) &&
				msgState.deliverySampleStat[value] >= p.deliveryThreshold {
				p.deliver(bInstance, value)
			}
		}
	}
}

func (p *Process) Receive(context actor.Context) {
	switch message := context.Message().(type) {
	case *actor.Started:
		p.logger.OnStart()
		context.RequestWithCustomSender(p.mainServer, &messages.Started{}, p.actorPid)
	case *actor.Stop:
		p.logger.OnStop()
	case *messages.Simulate:
		p.transactionManager.Simulate(context, p)
	case *messages.BroadcastInstanceMessage:
		bInstance := message.BroadcastInstance

		senderPid := utils.MakeCustomPid(context.Sender())
		p.logger.OnMessageReceived(senderPid, message.Stamp)

		switch protocolMessage := message.Message.(type) {
		case *messages.BroadcastInstanceMessage_ScalableProtocolMessage:
			p.processProtocolMessage(
				context,
				senderPid,
				bInstance,
				protocolMessage.ScalableProtocolMessage,
			)
		default:
			p.logger.Fatal(fmt.Sprintf("Invalid protocol message type %t", protocolMessage))
		}
	}
}

func (p *Process) Broadcast(context actor.SenderContext, value int64) {
	broadcastInstance := &messages.BroadcastInstance{
		Author:    p.pid,
		SeqNumber: p.transactionCounter,
	}

	msgState := p.initMessageState(context, broadcastInstance, value)
	p.broadcastGossip(context, msgState, broadcastInstance, value)

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}
