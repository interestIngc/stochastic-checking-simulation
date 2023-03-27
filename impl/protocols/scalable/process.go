package scalable

import (
	"github.com/asynkron/protoactor-go/actor"
	xrand "golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
	"log"
	"math/rand"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/manager"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/utils"
	"time"
)

type messageState struct {
	receivedEcho  map[string]bool
	receivedReady map[string]map[protocols.ValueType]bool

	echoMessagesStat   map[protocols.ValueType]int
	readySampleStat    map[protocols.ValueType]int
	deliverySampleStat map[protocols.ValueType]int

	sentReadyMessages map[protocols.ValueType]bool

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
	ms.receivedReady = make(map[string]map[protocols.ValueType]bool)

	ms.echoMessagesStat = make(map[protocols.ValueType]int)
	ms.readySampleStat = make(map[protocols.ValueType]int)
	ms.deliverySampleStat = make(map[protocols.ValueType]int)

	ms.sentReadyMessages = make(map[protocols.ValueType]bool)

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

	deliveredMessages map[string]map[int64]protocols.ValueType
	messagesLog       map[string]map[int64]*messageState

	gossipSampleSize   int
	echoSampleSize     int
	echoThreshold      int
	readySampleSize    int
	readyThreshold     int
	deliverySampleSize int
	deliveryThreshold  int

	transactionManager *manager.TransactionManager
	logger             *eventlogger.EventLogger
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
	sourceMessage *messages.SourceMessage,
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
		&messages.ScalableProtocolMessage{
			Stage:         stage,
			SourceMessage: sourceMessage,
		})

	return sample
}

func (p *Process) InitProcess(
	actorPid *actor.PID,
	actorPids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger) {
	p.actorPid = actorPid
	p.pid = utils.MakeCustomPid(actorPid)
	p.pids = make([]string, len(actorPids))
	p.actorPids = make(map[string]*actor.PID)

	p.transactionCounter = 0
	p.messageCounter = 0

	p.deliveredMessages = make(map[string]map[int64]protocols.ValueType)
	p.messagesLog = make(map[string]map[int64]*messageState)

	for i, currActorPid := range actorPids {
		pid := utils.MakeCustomPid(currActorPid)
		p.pids[i] = pid
		p.actorPids[pid] = currActorPid
		p.deliveredMessages[pid] = make(map[int64]protocols.ValueType)
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
}

func (p *Process) initMessageState(
	context actor.SenderContext,
	sourceMessage *messages.SourceMessage) *messageState {
	msgState := p.messagesLog[sourceMessage.Author][sourceMessage.SeqNumber]

	if msgState == nil {
		msgState = newMessageState()

		for _, pid := range p.pids {
			msgState.receivedReady[pid] = make(map[protocols.ValueType]bool)
		}

		msgState.gossipSample = p.generateGossipSample()
		p.broadcastToSet(
			context,
			msgState.gossipSample,
			&messages.ScalableProtocolMessage{
				Stage:         messages.ScalableProtocolMessage_GOSSIP_SUBSCRIBE,
				SourceMessage: sourceMessage,
			})

		msgState.echoSample =
			p.sample(
				context,
				messages.ScalableProtocolMessage_ECHO_SUBSCRIBE,
				sourceMessage,
				p.echoSampleSize)
		msgState.readySample =
			p.sample(
				context,
				messages.ScalableProtocolMessage_READY_SUBSCRIBE,
				sourceMessage,
				p.readySampleSize)
		msgState.deliverySample =
			p.sample(
				context,
				messages.ScalableProtocolMessage_READY_SUBSCRIBE,
				sourceMessage,
				p.deliverySampleSize)

		p.messagesLog[sourceMessage.Author][sourceMessage.SeqNumber] = msgState
	}

	return msgState
}

func (p *Process) sendMessage(
	context actor.SenderContext,
	to *actor.PID,
	message *messages.ScalableProtocolMessage) {
	message.Stamp = p.messageCounter

	context.RequestWithCustomSender(to, message, p.actorPid)

	p.logger.OnMessageSent(p.messageCounter)
	p.messageCounter++
}

func (p *Process) broadcastToSet(
	context actor.SenderContext,
	set map[string]bool,
	msg *messages.ScalableProtocolMessage) {
	for pid := range set {
		p.sendMessage(context, p.actorPids[pid], msg)
	}
}

func (p *Process) broadcastGossip(
	context actor.SenderContext,
	msgState *messageState,
	sourceMessage *messages.SourceMessage) {
	msgState.gossipMessage =
		&messages.ScalableProtocolMessage{
			Stage:         messages.ScalableProtocolMessage_GOSSIP,
			SourceMessage: sourceMessage,
		}
	p.broadcastToSet(
		context,
		msgState.gossipSample,
		msgState.gossipMessage)
}

func (p *Process) broadcastReady(
	context actor.SenderContext,
	msgState *messageState,
	sourceMessage *messages.SourceMessage) {
	value := protocols.ValueType(sourceMessage.Value)
	msgState.sentReadyMessages[value] = true

	p.broadcastToSet(
		context,
		msgState.readySubscriptionSet,
		&messages.ScalableProtocolMessage{
			Stage:         messages.ScalableProtocolMessage_READY,
			SourceMessage: sourceMessage,
		})
}

func (p *Process) delivered(sourceMessage *messages.SourceMessage) bool {
	deliveredValue, delivered :=
		p.deliveredMessages[sourceMessage.Author][sourceMessage.SeqNumber]

	if delivered && deliveredValue != protocols.ValueType(sourceMessage.Value) {
		p.logger.OnAttack(sourceMessage, int64(deliveredValue))
	}

	return delivered
}

func (p *Process) deliver(sourceMessage *messages.SourceMessage) {
	p.deliveredMessages[sourceMessage.Author][sourceMessage.SeqNumber] =
		protocols.ValueType(sourceMessage.Value)
	messagesReceived :=
		p.messagesLog[sourceMessage.Author][sourceMessage.SeqNumber].receivedMessagesCnt

	p.logger.OnDeliver(sourceMessage, messagesReceived)
}

func (p *Process) maybeSendReadyFromSieve(
	context actor.SenderContext,
	msgState *messageState,
	sourceMessage *messages.SourceMessage) {
	value := protocols.ValueType(sourceMessage.Value)

	if !msgState.sentReadyFromSieve &&
		msgState.echoMessage != nil &&
		value == protocols.ValueType(msgState.echoMessage.SourceMessage.Value) &&
		msgState.echoMessagesStat[value] >= p.echoThreshold {
		p.broadcastReady(context, msgState, sourceMessage)

		msgState.sentReadyFromSieve = true
	}
}

func (p *Process) Receive(context actor.Context) {
	message := context.Message()
	switch message.(type) {
	case *messages.Simulate:
		msg := message.(*messages.Simulate)

		p.transactionManager = &manager.TransactionManager{
			TransactionsToSendOut: msg.Transactions,
		}
		p.transactionManager.SendOutTransaction(context, p)
	case *messages.ScalableProtocolMessage:
		msg := message.(*messages.ScalableProtocolMessage)
		sourceMessage := msg.SourceMessage
		value := protocols.ValueType(sourceMessage.Value)

		sender := context.Sender()
		senderPid := utils.MakeCustomPid(sender)

		p.logger.OnMessageReceived(senderPid, msg.Stamp)

		msgState := p.initMessageState(context, sourceMessage)
		msgState.receivedMessagesCnt++

		switch msg.Stage {
		case messages.ScalableProtocolMessage_GOSSIP_SUBSCRIBE:
			if msgState.gossipSample[senderPid] {
				return
			}

			msgState.gossipSample[senderPid] = true
			if msgState.gossipMessage != nil {
				p.sendMessage(context, sender, msgState.gossipMessage)
			}
		case messages.ScalableProtocolMessage_GOSSIP:
			if msgState.gossipMessage == nil {
				p.broadcastGossip(context, msgState, sourceMessage)
			}
			if msgState.echoMessage == nil {
				msgState.echoMessage =
					&messages.ScalableProtocolMessage{
						Stage:         messages.ScalableProtocolMessage_ECHO,
						SourceMessage: sourceMessage,
					}
				p.broadcastToSet(
					context,
					msgState.echoSubscriptionSet,
					msgState.echoMessage)
				p.maybeSendReadyFromSieve(context, msgState, sourceMessage)
			}
		case messages.ScalableProtocolMessage_ECHO_SUBSCRIBE:
			if msgState.echoSubscriptionSet[senderPid] {
				return
			}

			msgState.echoSubscriptionSet[senderPid] = true
			if msgState.echoMessage != nil {
				p.sendMessage(context, sender, msgState.echoMessage)
			}
		case messages.ScalableProtocolMessage_ECHO:
			if !msgState.echoSample[senderPid] || msgState.receivedEcho[senderPid] {
				return
			}

			msgState.receivedEcho[senderPid] = true
			msgState.echoMessagesStat[value]++

			p.maybeSendReadyFromSieve(context, msgState, sourceMessage)
		case messages.ScalableProtocolMessage_READY_SUBSCRIBE:
			if msgState.readySubscriptionSet[senderPid] {
				return
			}

			msgState.readySubscriptionSet[senderPid] = true

			for val := range msgState.sentReadyMessages {
				p.sendMessage(
					context,
					sender,
					&messages.ScalableProtocolMessage{
						Stage: messages.ScalableProtocolMessage_READY,
						SourceMessage: &messages.SourceMessage{
							Author:    sourceMessage.Author,
							SeqNumber: sourceMessage.SeqNumber,
							Value:     int64(val),
						},
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
					p.broadcastReady(context, msgState, sourceMessage)
				}
			}

			if msgState.deliverySample[senderPid] {
				msgState.deliverySampleStat[value]++

				if !p.delivered(sourceMessage) &&
					msgState.deliverySampleStat[value] >= p.deliveryThreshold {
					p.deliver(sourceMessage)
				}
			}
		}
	}
}

func (p *Process) Broadcast(context actor.SenderContext, value int64) {
	sourceMessage := &messages.SourceMessage{
		Author:    p.pid,
		SeqNumber: p.transactionCounter,
		Value:     value,
	}

	msgState := p.initMessageState(context, sourceMessage)
	p.broadcastGossip(context, msgState, sourceMessage)

	p.logger.OnTransactionInit(p.transactionCounter)

	p.transactionCounter++
}
