package reliable

import (
	"crypto/rsa"
	"fmt"
	"github.com/klauspost/reedsolomon"
	"math"
	"math/rand"
	"stochastic-checking-simulation/context"
	"stochastic-checking-simulation/impl/eventlogger"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/utils"
	"time"
)

type ProcessId int32

type WitnessStage int

const (
	InitialWitnessStage WitnessStage = iota
	SentEchoFromWitness
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
	SentReveal
	SentDoneToWitnesses
	SentDoneToProcesses
	SentFailedToProcesses
	SentFailedToWitnesses
)

const bytes = 4

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

	encryptedShares   [][]byte
	ownEncryptedShare []byte

	decryptedShares [][]byte

	receivedMessagesCnt int
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

	return ms
}

type Process struct {
	processIndex int32
	actorPids    map[string]ProcessId
	pids         []string

	encoder reedsolomon.Encoder

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
	parityShares int
	processCount int

	MixingTime int

	wSelector     *hashing.WitnessesSelector
	historyHashes []*hashing.HistoryHash

	context                      *context.ReliableContext
	logger                       *eventlogger.EventLogger
	ownDeliveredTransactions     chan bool
	sendOwnDeliveredTransactions bool

	PublicKeys []*rsa.PublicKey
	PrivateKey *rsa.PrivateKey
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
	p.parityShares = parameters.ProcessCount - p.dataShares
	p.processCount = parameters.ProcessCount

	var err error
	p.encoder, err = reedsolomon.New(p.dataShares, p.parityShares)
	if err != nil {
		p.logger.Fatal("Could not instantiate the reed-solomon encoder")
	}

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

	p.historyHashes = make([]*hashing.HistoryHash, parameters.ProcessCount)
	for i := 0; i < parameters.ProcessCount; i++ {
		p.historyHashes[i] =
			hashing.NewHistoryHash(uint(parameters.NumberOfBins), binCapacity, hasher, int32(i))
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
		p.wSelector.GetWitnessSet(p.pids, p.historyHashes)

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

	transactionToReveal, ok := p.checkpoints[p.deliveredMessagesCount-p.MixingTime]
	if ok {
		p.startRevealPhase(transactionToReveal)

		delete(p.checkpoints, p.deliveredMessagesCount-p.MixingTime)
	}

	msgState := p.messagesLog[author][bInstance.SeqNumber]
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

	decryptedShare := decrypt(p.PrivateKey, msgState.ownEncryptedShare, p.logger)

	p.broadcastCommitmentToWitnesses(
		transaction,
		&messages.CommitmentProtocolMessage{
			Stage:          messages.CommitmentProtocolMessage_REVEAL,
			DecryptedShare: decryptedShare,
		},
		msgState,
	)
}

func (p *Process) finallyCommitted(
	bInstance *messages.BroadcastInstance,
) bool {
	_, committed :=
		p.finallyCommittedMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber]

	return committed
}

func (p *Process) processReliableProtocolMessage(
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	reliableMessage *messages.ReliableProtocolMessage,
) {
	broadcastMessage := reliableMessage.BroadcastMessage

	if p.finallyCommitted(bInstance) {
		return
	}

	msgState := p.registerMessage(bInstance)
	msgState.receivedMessagesCnt++

	senderPid := p.pids[senderId]

	switch reliableMessage.Stage {
	case messages.ReliableProtocolMessage_NOTIFY:
		if !p.isWitness(msgState) || msgState.witnessStage >= SentEchoFromWitness {
			return
		}

		encryptedShares := reliableMessage.EncryptedShares
		msgState.encryptedShares = encryptedShares

		p.logger.Println("Received shares: " + fmt.Sprint(encryptedShares))

		for i := 0; i < p.processCount; i++ {
			share := encryptedShares[i]
			p.sendProtocolMessage(
				ProcessId(i),
				bInstance,
				&messages.ReliableProtocolMessage{
					Stage:            messages.ReliableProtocolMessage_ECHO_FROM_WITNESS,
					BroadcastMessage: broadcastMessage,
					EncryptedShares:  [][]byte{share},
				},
			)
		}

		msgState.witnessStage = SentEchoFromWitness
	case messages.ReliableProtocolMessage_ECHO_FROM_WITNESS:
		if !msgState.ownWitnessSet[senderPid] || msgState.stage >= SentEchoFromProcess {
			return
		}

		msgState.ownEncryptedShare = reliableMessage.EncryptedShares[0]

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

func (p *Process) addSecret(secret []int32) {
	secretBytes := make([]byte, bytes*len(secret))
	for i := 0; i < len(secret); i++ {
		currBytes := utils.Int32ToBytes(secret[i])
		for j, currByte := range currBytes {
			secretBytes[i*bytes+j] = currByte
		}
	}

	for i := 0; i < p.processCount; i++ {
		p.historyHashes[i].Insert(secretBytes)
	}
}

func (p *Process) processCommitmentProtocolMessage(
	senderId ProcessId,
	bInstance *messages.BroadcastInstance,
	commitmentMessage *messages.CommitmentProtocolMessage,
) {
	if p.finallyCommitted(bInstance) {
		return
	}

	msgState := p.registerMessage(bInstance)
	msgState.receivedMessagesCnt++

	senderPid := p.pids[senderId]
	deliveredBroadcast := p.deliveredMessages[ProcessId(bInstance.Author)][bInstance.SeqNumber]

	switch commitmentMessage.Stage {
	case messages.CommitmentProtocolMessage_REVEAL:
		if !p.isWitness(msgState) ||
			msgState.revealFromProcesses[senderId] ||
			msgState.revealStage == SentDoneToProcesses ||
			msgState.revealStage == SentFailedToProcesses {
			return
		}

		msgState.revealFromProcesses[senderId] = true

		if msgState.decryptedShares == nil {
			msgState.decryptedShares = make([][]byte, p.processCount)
		}
		decryptedShare := commitmentMessage.DecryptedShare
		msgState.decryptedShares[senderId] = decryptedShare

		encryptedShare := encrypt(p.PublicKeys[senderId], decryptedShare, p.logger)
		if !utils.AreEqual(encryptedShare, msgState.encryptedShares[senderId]) {
			p.logger.Fatal(
				fmt.Sprintf(
					"Non-equal encrypted shares detected: expected %d, got %d from sender %d",
					msgState.encryptedShares[senderId],
					encryptedShare,
					senderId,
				),
			)
		}

		if len(msgState.revealFromProcesses) >= 2*p.faultyProcesses+1 {
			err := p.encoder.Reconstruct(msgState.decryptedShares)
			if err != nil {
				p.logger.Fatal("Error while decoding decrypted shares: " + err.Error())
			}

			xPrime := make([]int32, p.dataShares)
			for i := 0; i < p.dataShares; i++ {
				xPrime[i] = utils.ToInt32(msgState.decryptedShares[i])
			}

			xPrimeHash := hash(xPrime)

			if xPrimeHash == deliveredBroadcast.XHash {
				p.broadcastCommitmentMessage(
					bInstance,
					&messages.CommitmentProtocolMessage{
						Stage: messages.CommitmentProtocolMessage_DONE,
						X:     xPrime,
					},
				)
				msgState.revealStage = SentDoneToProcesses
			} else {
				p.logger.Fatal(
					fmt.Sprintf(
						"X hash mismatch: delivered %d, received %d",
						deliveredBroadcast.XHash, xPrimeHash,
					),
				)
				//p.broadcastCommitmentMessage(
				//	bInstance,
				//	&messages.CommitmentProtocolMessage{
				//		Stage: messages.CommitmentProtocolMessage_FAILED,
				//	},
				//)
				//msgState.revealStage = SentFailedToProcesses
			}
		}
	case messages.CommitmentProtocolMessage_DONE:
		if hash(commitmentMessage.X) == deliveredBroadcast.XHash {
			if p.isWitness(msgState) && msgState.revealStage != SentDoneToProcesses {
				p.broadcastCommitmentMessage(
					bInstance,
					&messages.CommitmentProtocolMessage{
						Stage: messages.CommitmentProtocolMessage_DONE,
						X:     commitmentMessage.X,
					},
				)
				msgState.revealStage = SentDoneToProcesses
			}

			checkpoint := p.transactionToCheckpoint[ProcessId(bInstance.Author)][bInstance.SeqNumber]
			if msgState.ownWitnessSet[senderPid] &&
				p.deliveredMessagesCount-checkpoint >= p.MixingTime {
				if msgState.revealStage != SentDoneToWitnesses {
					p.broadcastCommitmentToWitnesses(
						bInstance,
						&messages.CommitmentProtocolMessage{
							Stage: messages.CommitmentProtocolMessage_DONE,
							X:     commitmentMessage.X,
						},
						msgState,
					)
					msgState.revealStage = SentDoneToWitnesses
				}
				p.addSecret(commitmentMessage.X)
				p.cleanUp(bInstance, deliveredBroadcast.Value)
			}
		}
	case messages.CommitmentProtocolMessage_FAILED:
		if msgState.ownWitnessSet[senderPid] &&
			msgState.revealStage != SentFailedToWitnesses {
			p.broadcastCommitmentToWitnesses(
				bInstance,
				&messages.CommitmentProtocolMessage{
					Stage: messages.CommitmentProtocolMessage_FAILED,
				},
				msgState,
			)
			msgState.revealStage = SentFailedToWitnesses
		}

		if p.isWitness(msgState) &&
			msgState.revealStage != SentFailedToProcesses {
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

func (p *Process) Broadcast(
	value int32,
) {
	broadcastInstance := &messages.BroadcastInstance{
		Author:    p.processIndex,
		SeqNumber: p.transactionCounter,
	}

	x := p.sample()
	xHash := hash(x)

	data, err := p.encodeEntropy(x)

	if err != nil {
		p.logger.Fatal("Error when encoding data using RS: " + err.Error())
	}

	encryptedShares := make([][]byte, p.processCount)
	for i := 0; i < p.processCount; i++ {
		encryptedShares[i] = encrypt(p.PublicKeys[i], data[i], p.logger)
	}
	p.logger.Println("Broadcasting: " + fmt.Sprint(encryptedShares))

	msgState := p.initMessageState(broadcastInstance)

	p.broadcastToWitnesses(
		broadcastInstance,
		&messages.ReliableProtocolMessage{
			Stage: messages.ReliableProtocolMessage_NOTIFY,
			BroadcastMessage: &messages.Broadcast{
				Value: value,
				XHash: xHash,
			},
			EncryptedShares: encryptedShares,
		},
		msgState)

	p.logger.OnTransactionInit(broadcastInstance)

	p.transactionCounter++
}

func (p *Process) sample() []int32 {
	sample := make([]int32, p.dataShares)
	uniform := rand.New(rand.NewSource(int64(p.processIndex)))

	for i := 0; i < p.dataShares; i++ {
		sample[i] = uniform.Int31()
	}

	return sample
}

func (p *Process) encodeEntropy(x []int32) ([][]byte, error) {
	data := make([][]byte, p.processCount)

	for i := 0; i < p.dataShares; i++ {
		data[i] = utils.Int32ToBytes(x[i])
	}

	for i := p.dataShares; i < p.processCount; i++ {
		data[i] = make([]byte, bytes)
	}

	err := p.encoder.Encode(data)

	return data, err
}
