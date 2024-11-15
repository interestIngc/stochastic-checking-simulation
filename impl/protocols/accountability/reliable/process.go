package reliable

import (
	"crypto/sha256"
	"fmt"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/schnorr"
	sss "go.openfort.xyz/shamir-secret-sharing-go"
	"math"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"strconv"
	"time"
)

type ProcessId int32

type WitnessStage int

const (
	InitialWitnessStage WitnessStage = iota
	SentReadyFromWitness
	SentValidate
)

type Stage int

const (
	InitialStage Stage = iota
	SentEchoFromProcess
	SentReadyFromProcess
	Delivered
)

type RevealStage int

const (
	InitialRevealStage RevealStage = iota
	SentDoneToWitnesses
	SentDoneToProcesses
	SentFailedToProcesses
	SentFailedToWitnesses
)

const historyLines int32 = 8

type messageState struct {
	echoFromProcesses     map[ProcessId]bool
	readyFromProcesses    map[ProcessId]bool
	readyFromWitnesses    map[ProcessId]bool
	validateFromWitnesses map[ProcessId]bool
	revealFromProcesses   map[ProcessId]bool

	echoFromProcessesStat  map[int32]int
	readyFromProcessesStat map[int32]int
	readyFromWitnessesStat map[int32]int
	validatesStat          map[int32]int

	stage        Stage
	witnessStage WitnessStage
	revealStage  RevealStage

	ownWitnessSet map[string]bool
	potWitnessSet map[string]bool

	ownShare []byte
	shares   [][]byte

	receivedMessagesCnt int

	pendingRevealProtocolMessages []*CommitmentMessageAndSender
}

func newMessageState() *messageState {
	ms := new(messageState)

	ms.echoFromProcesses = make(map[ProcessId]bool)
	ms.readyFromProcesses = make(map[ProcessId]bool)
	ms.readyFromWitnesses = make(map[ProcessId]bool)
	ms.validateFromWitnesses = make(map[ProcessId]bool)
	ms.revealFromProcesses = make(map[ProcessId]bool)

	ms.echoFromProcessesStat = make(map[int32]int)
	ms.readyFromWitnessesStat = make(map[int32]int)
	ms.readyFromProcessesStat = make(map[int32]int)
	ms.validatesStat = make(map[int32]int)

	ms.stage = InitialStage
	ms.witnessStage = InitialWitnessStage
	ms.revealStage = InitialRevealStage

	ms.receivedMessagesCnt = 0

	ms.pendingRevealProtocolMessages = make([]*CommitmentMessageAndSender, 0)

	return ms
}

type Process struct {
	processIndex int32
	actorPids    map[string]ProcessId
	pids         []string

	transactionCounter int32

	deliveredMessages        map[ProcessId]map[int32]*messages.Broadcast
	messagesLog              map[ProcessId]map[int32]*messageState
	checkpoints              map[int]*messages.BroadcastInstance
	finallyCommittedMessages map[ProcessId]map[int32]int32
	transactionToCheckpoint  map[ProcessId]map[int32]int

	deliveredMessagesCount int

	quorumThreshold         int
	readyMessagesThreshold  int
	recoverySwitchTimeoutNs time.Duration
	witnessThreshold        int
	faultyProcesses         int

	dataShares   int
	processCount int

	MixingTime int

	wSelector     *hashing.WitnessesSelector
	historyHashes [][]*hashing.HistoryHash

	context                      *context.ReliableContext
	logger                       *eventlogger.EventLogger
	ownDeliveredTransactions     chan bool
	sendOwnDeliveredTransactions bool

	PublicKeys []*secp256k1.PublicKey
	PrivateKey *secp256k1.PrivateKey
}

func (p *Process) InitProcess(
	processIndex int32,
	actorPids []string,
	parameters *parameters.Parameters,
	context *context.ReliableContext,
	logger *eventlogger.EventLogger,
	ownDeliveredTransactions chan bool,
	sendOwnDeliveredTransactions bool,
) {
	p.processIndex = processIndex
	p.pids = actorPids

	p.transactionCounter = 0
	p.deliveredMessagesCount = 0

	p.quorumThreshold = int(math.Ceil(float64(len(actorPids)+parameters.FaultyProcesses+1) / float64(2)))
	p.readyMessagesThreshold = parameters.FaultyProcesses + 1
	p.recoverySwitchTimeoutNs = time.Duration(parameters.RecoverySwitchTimeoutNs)
	p.witnessThreshold = parameters.WitnessThreshold
	p.faultyProcesses = parameters.FaultyProcesses

	p.dataShares = parameters.FaultyProcesses + 1
	p.processCount = parameters.ProcessCount

	p.actorPids = make(map[string]ProcessId)
	p.deliveredMessages = make(map[ProcessId]map[int32]*messages.Broadcast)
	p.messagesLog = make(map[ProcessId]map[int32]*messageState)
	p.checkpoints = make(map[int]*messages.BroadcastInstance)
	p.finallyCommittedMessages = make(map[ProcessId]map[int32]int32)
	p.transactionToCheckpoint = make(map[ProcessId]map[int32]int)

	for i, pid := range actorPids {
		p.actorPids[pid] = ProcessId(i)
		p.deliveredMessages[ProcessId(i)] = make(map[int32]*messages.Broadcast)
		p.messagesLog[ProcessId(i)] = make(map[int32]*messageState)
		p.finallyCommittedMessages[ProcessId(i)] = make(map[int32]int32)
		p.transactionToCheckpoint[ProcessId(i)] = make(map[int32]int)
	}

	binCapacity := uint(math.Pow(2, float64(parameters.NodeIdSize/parameters.NumberOfBins)))

	var hasher hashing.Hasher
	if parameters.NodeIdSize == 256 {
		hasher = hashing.HashSHA256{}
	} else {
		hasher = hashing.HashSHA512{}
	}

	p.wSelector = &hashing.WitnessesSelector{
		MinPotWitnessSetSize: parameters.MinPotWitnessSetSize,
		MinOwnWitnessSetSize: parameters.MinOwnWitnessSetSize,
		PotWitnessSetRadius:  parameters.PotWitnessSetRadius,
		OwnWitnessSetRadius:  parameters.OwnWitnessSetRadius,
	}

	p.historyHashes = make([][]*hashing.HistoryHash, historyLines)
	for i := 0; i < len(p.historyHashes); i++ {
		p.historyHashes[i] = make([]*hashing.HistoryHash, parameters.ProcessCount)
		for j := 0; j < parameters.ProcessCount; j++ {
			p.historyHashes[i][j] = hashing.NewHistoryHash(
				uint(parameters.NumberOfBins),
				binCapacity,
				hasher,
				int32(j+i*parameters.ProcessCount),
			)
		}
	}

	p.context = context
	p.logger = logger
	p.ownDeliveredTransactions = ownDeliveredTransactions
	p.sendOwnDeliveredTransactions = sendOwnDeliveredTransactions
}

func (p *Process) initMessageState(
	bInstance *messages.BroadcastInstance,
) *messageState {
	msgState := newMessageState()
	p.messagesLog[ProcessId(bInstance.Author)][bInstance.SeqNumber] = msgState

	msgState.ownWitnessSet, msgState.potWitnessSet =
		p.wSelector.GetWitnessSet(p.pids, p.historyHashes[bInstance.Author%historyLines])

	p.logger.OnWitnessSetSelected("own", bInstance, msgState.ownWitnessSet)
	p.logger.OnWitnessSetSelected("pot", bInstance, msgState.potWitnessSet)

	return msgState
}

func (p *Process) registerMessage(
	bInstance *messages.BroadcastInstance,
) *messageState {
	msgState := p.messagesLog[ProcessId(bInstance.Author)][bInstance.SeqNumber]
	if msgState == nil {
		msgState = p.initMessageState(bInstance)
	}
	return msgState
}

func (p *Process) sendMessage(
	to ProcessId,
	bMessage *messages.BroadcastInstanceMessage,
) {
	msg := p.context.MakeNewMessage()
	msg.Content = &messages.Message_BroadcastInstanceMessage{
		BroadcastInstanceMessage: bMessage,
	}

	p.context.Send(int32(to), msg)
}

func (p *Process) sendProtocolMessage(
	to ProcessId,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_ReliableProtocolMessage{
			ReliableProtocolMessage: reliableMessage,
		},
	}
	p.sendMessage(to, bMessage)
}

func (p *Process) sendCommitmentMessage(
	to ProcessId,
	bInstance *messages.BroadcastInstance,
	commitmentMessage *messages.CommitmentProtocolMessage,
) {
	bMessage := &messages.BroadcastInstanceMessage{
		BroadcastInstance: bInstance,
		Message: &messages.BroadcastInstanceMessage_CommitmentProtocolMessage{
			CommitmentProtocolMessage: commitmentMessage,
		},
	}
	p.sendMessage(to, bMessage)
}

func (p *Process) broadcastProtocolMessage(
	bInstance *messages.BroadcastInstance,
	message *messages.ReliableProtocolMessage,
) {
	for i := range p.pids {
		p.sendProtocolMessage(ProcessId(i), bInstance, message)
	}
}

func (p *Process) broadcastCommitmentMessage(
	bInstance *messages.BroadcastInstance,
	message *messages.CommitmentProtocolMessage,
) {
	for i := range p.pids {
		p.sendCommitmentMessage(ProcessId(i), bInstance, message)
	}
}

func (p *Process) broadcastToWitnesses(
	bInstance *messages.BroadcastInstance,
	message *messages.ReliableProtocolMessage,
	msgState *messageState,
) {
	for pid := range msgState.potWitnessSet {
		p.sendProtocolMessage(p.actorPids[pid], bInstance, message)
	}
}

func (p *Process) broadcastCommitmentToWitnesses(
	bInstance *messages.BroadcastInstance,
	message *messages.CommitmentProtocolMessage,
	msgState *messageState,
) {
	for pid := range msgState.potWitnessSet {
		p.sendCommitmentMessage(p.actorPids[pid], bInstance, message)
	}
}

func (p *Process) broadcastReadyFromWitness(
	bInstance *messages.BroadcastInstance,
	broadcastMessage *messages.Broadcast,
	msgState *messageState,
) {
	p.broadcastProtocolMessage(
		bInstance,
		&messages.ReliableProtocolMessage{
			Stage:            messages.ReliableProtocolMessage_READY_FROM_WITNESS,
			BroadcastMessage: broadcastMessage,
		})
	msgState.witnessStage = SentReadyFromWitness
}

func (p *Process) isWitness(msgState *messageState) bool {
	return msgState.potWitnessSet[p.pids[p.processIndex]]
}

func (p *Process) deliver(
	bInstance *messages.BroadcastInstance,
	broadcastMessage *messages.Broadcast,
) {
	author := ProcessId(bInstance.Author)
	p.deliveredMessages[author][bInstance.SeqNumber] = broadcastMessage
	p.deliveredMessagesCount++

	p.checkpoints[p.deliveredMessagesCount] = bInstance
	p.transactionToCheckpoint[ProcessId(bInstance.Author)][bInstance.SeqNumber] =
		p.deliveredMessagesCount

	msgState := p.messagesLog[author][bInstance.SeqNumber]

	for _, msgAndSender := range msgState.pendingRevealProtocolMessages {
		p.processCommitmentProtocolMessage(
			msgAndSender.sender,
			bInstance,
			msgAndSender.message,
		)
	}
	msgState.pendingRevealProtocolMessages = nil

	transactionToReveal, ok := p.checkpoints[p.deliveredMessagesCount-p.MixingTime]
	if ok {
		p.startRevealPhase(transactionToReveal)
		delete(p.checkpoints, p.deliveredMessagesCount-p.MixingTime)
	}

	p.logger.OnDeliver(bInstance, broadcastMessage.Value, msgState.receivedMessagesCnt)

	if p.sendOwnDeliveredTransactions && bInstance.Author == p.processIndex {
		p.ownDeliveredTransactions <- true
	}
}

func (p *Process) cleanUp(bInstance *messages.BroadcastInstance, value int32) {
	delete(p.messagesLog[ProcessId(bInstance.Author)], bInstance.SeqNumber)

	p.finallyCommittedMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber] = value
}

func (p *Process) startRevealPhase(transaction *messages.BroadcastInstance) {
	msgState := p.messagesLog[ProcessId(transaction.Author)][transaction.SeqNumber]

	p.broadcastCommitmentToWitnesses(
		transaction,
		&messages.CommitmentProtocolMessage{
			Stage: messages.CommitmentProtocolMessage_REVEAL,
			Share: msgState.ownShare,
		},
		msgState,
	)
}

func (p *Process) isFinallyCommitted(bInstance *messages.BroadcastInstance) bool {
	_, committed := p.finallyCommittedMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber]
	return committed
}

func (p *Process) processReliableProtocolMessage(
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
) {
	broadcastMessage := reliableMessage.BroadcastMessage

	if p.isFinallyCommitted(bInstance) {
		return
	}

	msgState := p.registerMessage(bInstance)
	msgState.receivedMessagesCnt++

	senderPid := p.pids[senderId]

	switch reliableMessage.Stage {
	case messages.ReliableProtocolMessage_NOTIFY:
		if msgState.stage >= SentEchoFromProcess {
			return
		}

		msgState.ownShare = reliableMessage.Share
		p.broadcastToWitnesses(
			bInstance,
			&messages.ReliableProtocolMessage{
				Stage:            messages.ReliableProtocolMessage_ECHO_FROM_PROCESS,
				BroadcastMessage: broadcastMessage,
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
		msgState.echoFromProcessesStat[broadcastMessage.Value]++

		if msgState.echoFromProcessesStat[broadcastMessage.Value] >= p.quorumThreshold {
			p.broadcastReadyFromWitness(
				bInstance,
				broadcastMessage,
				msgState,
			)
		}
	case messages.ReliableProtocolMessage_READY_FROM_WITNESS:
		if !msgState.ownWitnessSet[senderPid] ||
			msgState.stage >= SentReadyFromProcess ||
			msgState.readyFromWitnesses[senderId] {
			return
		}

		msgState.readyFromWitnesses[senderId] = true
		msgState.readyFromWitnessesStat[broadcastMessage.Value]++

		if msgState.readyFromWitnessesStat[broadcastMessage.Value] >= p.witnessThreshold {
			p.broadcastToWitnesses(
				bInstance,
				&messages.ReliableProtocolMessage{
					Stage:            messages.ReliableProtocolMessage_READY_FROM_PROCESS,
					BroadcastMessage: broadcastMessage,
				},
				msgState,
			)
			msgState.stage = SentReadyFromProcess
		}
	case messages.ReliableProtocolMessage_READY_FROM_PROCESS:
		if !p.isWitness(msgState) || msgState.readyFromProcesses[senderId] {
			return
		}

		msgState.readyFromProcesses[senderId] = true
		msgState.readyFromProcessesStat[broadcastMessage.Value]++

		if msgState.witnessStage < SentReadyFromWitness &&
			msgState.readyFromProcessesStat[broadcastMessage.Value] >= p.readyMessagesThreshold {
			p.broadcastReadyFromWitness(
				bInstance,
				broadcastMessage,
				msgState,
			)
		}

		if msgState.witnessStage < SentValidate &&
			msgState.readyFromProcessesStat[broadcastMessage.Value] >= p.quorumThreshold {
			p.broadcastProtocolMessage(
				bInstance,
				&messages.ReliableProtocolMessage{
					Stage:            messages.ReliableProtocolMessage_VALIDATE,
					BroadcastMessage: broadcastMessage,
				},
			)
			msgState.witnessStage = SentValidate
		}
	case messages.ReliableProtocolMessage_VALIDATE:
		if !msgState.ownWitnessSet[senderPid] ||
			msgState.stage >= Delivered ||
			msgState.validateFromWitnesses[senderId] {
			return
		}

		msgState.validateFromWitnesses[senderId] = true
		msgState.validatesStat[broadcastMessage.Value]++

		if msgState.validatesStat[broadcastMessage.Value] >= p.witnessThreshold {
			p.deliver(bInstance, broadcastMessage)
			msgState.stage = Delivered
		}
	}
}

func (p *Process) addSecret(secret []byte, author int32) {
	for i := 0; i < p.processCount; i++ {
		p.historyHashes[author%historyLines][i].Insert(secret)
	}
}

func (p *Process) broadcastFailedToProcesses(
	bInstance *messages.BroadcastInstance,
	msgState *messageState,
	errorMessage string,
) {
	p.logger.Println(errorMessage)

	p.broadcastCommitmentMessage(
		bInstance,
		&messages.CommitmentProtocolMessage{
			Stage: messages.CommitmentProtocolMessage_FAILED,
		},
	)
	msgState.revealStage = SentFailedToProcesses
}

func (p *Process) processCommitmentProtocolMessage(
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	commitmentMessage *messages.CommitmentProtocolMessage,
) {
	if p.isFinallyCommitted(bInstance) {
		return
	}

	msgState := p.registerMessage(bInstance)
	msgState.receivedMessagesCnt++

	senderPid := p.pids[senderId]

	deliveredBroadcast, delivered :=
		p.deliveredMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber]

	if !delivered {
		msgState.pendingRevealProtocolMessages = append(
			msgState.pendingRevealProtocolMessages,
			&CommitmentMessageAndSender{
				sender:  senderId,
				message: commitmentMessage,
			},
		)
		return
	}

	switch commitmentMessage.Stage {
	case messages.CommitmentProtocolMessage_REVEAL:
		if !p.isWitness(msgState) ||
			msgState.revealFromProcesses[senderId] ||
			msgState.revealStage == SentDoneToProcesses ||
			msgState.revealStage == SentFailedToProcesses {
			return
		}

		msgState.revealFromProcesses[senderId] = true

		if msgState.shares == nil {
			msgState.shares = make([][]byte, p.processCount)
		}
		msgState.shares[senderId] = commitmentMessage.Share

		if len(msgState.revealFromProcesses) >= p.faultyProcesses+1 {
			gathered := make([][]byte, p.faultyProcesses+1)
			count := 0
			for i := 0; i < p.processCount; i++ {
				if msgState.shares[i] != nil {
					gathered[count] = msgState.shares[i]
					count++
				}
				if count == p.faultyProcesses+1 {
					break
				}
			}

			if count < p.faultyProcesses+1 {
				return
			}

			xPrime, err := sss.Combine(gathered)

			if err != nil {
				p.broadcastFailedToProcesses(
					bInstance,
					msgState,
					fmt.Sprintf("Error while decoding shares: %e", err),
				)
				return
			}

			messageHash := getTransactionHash(bInstance.Author, bInstance.SeqNumber)

			signature, err := schnorr.ParseSignature(xPrime)
			if err != nil {
				p.broadcastFailedToProcesses(
					bInstance,
					msgState,
					fmt.Sprintf("Error while parsing signature: %e", err),
				)
				return
			}

			verified := signature.Verify(messageHash[:], p.PublicKeys[int(bInstance.Author)])
			if !verified {
				p.broadcastFailedToProcesses(
					bInstance,
					msgState,
					fmt.Sprintf("Error while verifying signature: %e", err),
				)
				return
			}

			p.broadcastCommitmentMessage(
				bInstance,
				&messages.CommitmentProtocolMessage{
					Stage: messages.CommitmentProtocolMessage_DONE,
					Share: xPrime,
				},
			)
			msgState.revealStage = SentDoneToProcesses
		}
	case messages.CommitmentProtocolMessage_DONE:
		messageHash := getTransactionHash(bInstance.Author, bInstance.SeqNumber)

		signature, err := schnorr.ParseSignature(commitmentMessage.Share)
		if err != nil {
			return
		}

		verified := signature.Verify(messageHash[:], p.PublicKeys[int(bInstance.Author)])
		if verified {
			if p.isWitness(msgState) && msgState.revealStage != SentDoneToProcesses {
				p.broadcastCommitmentMessage(
					bInstance,
					&messages.CommitmentProtocolMessage{
						Stage: messages.CommitmentProtocolMessage_DONE,
						Share: commitmentMessage.Share,
					},
				)
				msgState.revealStage = SentDoneToProcesses
			}

			checkpoint := p.transactionToCheckpoint[ProcessId(bInstance.Author)][bInstance.SeqNumber]
			if p.deliveredMessagesCount-checkpoint >= p.MixingTime {
				if msgState.revealStage != SentDoneToWitnesses {
					p.broadcastCommitmentToWitnesses(
						bInstance,
						&messages.CommitmentProtocolMessage{
							Stage: messages.CommitmentProtocolMessage_DONE,
							Share: commitmentMessage.Share,
						},
						msgState,
					)
					msgState.revealStage = SentDoneToWitnesses
				}
				p.addSecret(commitmentMessage.Share, bInstance.Author)
				p.cleanUp(bInstance, deliveredBroadcast.Value)
			}
		}
	case messages.CommitmentProtocolMessage_FAILED:
		if msgState.ownWitnessSet[senderPid] && msgState.revealStage != SentFailedToWitnesses {
			p.broadcastCommitmentToWitnesses(
				bInstance,
				&messages.CommitmentProtocolMessage{
					Stage: messages.CommitmentProtocolMessage_FAILED,
				},
				msgState,
			)
			msgState.revealStage = SentFailedToWitnesses
		}

		if p.isWitness(msgState) && msgState.revealStage != SentFailedToProcesses {
			p.broadcastCommitmentMessage(
				bInstance,
				&messages.CommitmentProtocolMessage{
					Stage: messages.CommitmentProtocolMessage_FAILED,
				},
			)
			msgState.revealStage = SentFailedToProcesses
		}
	}
}

func (p *Process) HandleMessage(
	sender int32,
	broadcastInstanceMessage *messages.BroadcastInstanceMessage,
) {
	bInstance := broadcastInstanceMessage.BroadcastInstance
	senderId := ProcessId(sender)

	switch protocolMessage := broadcastInstanceMessage.Message.(type) {
	case *messages.BroadcastInstanceMessage_ReliableProtocolMessage:
		p.processReliableProtocolMessage(
			senderId,
			bInstance,
			protocolMessage.ReliableProtocolMessage,
		)
	case *messages.BroadcastInstanceMessage_CommitmentProtocolMessage:
		p.processCommitmentProtocolMessage(
			senderId,
			bInstance,
			protocolMessage.CommitmentProtocolMessage,
		)
	default:
		p.logger.Fatal(fmt.Sprintf("Invalid protocol message type %t", protocolMessage))
	}
}

func (p *Process) Broadcast(value int32) {
	broadcastInstance := &messages.BroadcastInstance{
		Author:    p.processIndex,
		SeqNumber: p.transactionCounter,
	}

	messageHash := getTransactionHash(p.processIndex, p.transactionCounter)

	signature, err := schnorr.Sign(p.PrivateKey, messageHash[:])
	if err != nil {
		p.logger.Fatal("Error when signing a message")
		return
	}

	shares, err := sss.Split(p.processCount, p.dataShares, signature.Serialize())
	if err != nil {
		p.logger.Fatal("Error when splitting data using SSS: " + err.Error())
		return
	}

	for i := 0; i < p.processCount; i++ {
		share := shares[i]
		p.sendProtocolMessage(
			ProcessId(i),
			broadcastInstance,
			&messages.ReliableProtocolMessage{
				Stage: messages.ReliableProtocolMessage_NOTIFY,
				BroadcastMessage: &messages.Broadcast{
					Value: value,
				},
				Share: share,
			},
		)
	}

	p.logger.OnTransactionInit(broadcastInstance)
	p.transactionCounter++
}

func getTransactionHash(author int32, seqNumber int32) [32]byte {
	messageString := strconv.Itoa(int(author)) + strconv.Itoa(int(seqNumber))
	return sha256.Sum256([]byte(messageString))
}

type CommitmentMessageAndSender struct {
	sender  ProcessId
	message *messages.CommitmentProtocolMessage
}
