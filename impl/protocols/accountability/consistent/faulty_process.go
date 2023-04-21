package consistent

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/protocols"
	"stochastic-checking-simulation/impl/utils"
)

type FaultyProcess struct {
	process *CorrectProcess
}

func (p *FaultyProcess) InitProcess(
	currPid *actor.PID,
	pids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger,
	transactionManager *protocols.TransactionManager,
	mainServer *actor.PID,
) {
	p.process = &CorrectProcess{}
	p.process.InitProcess(currPid, pids, parameters, logger, transactionManager, mainServer)
}

func (p *FaultyProcess) Receive(context actor.Context) {
	switch message := context.Message().(type) {
	case *messages.FaultyBroadcast:
		p.FaultyBroadcast(context, message.Value1, message.Value2)
	case *messages.BroadcastInstanceMessage:
		bInstance := message.BroadcastInstance

		switch protocolMessage := message.Message.(type) {
		case *messages.BroadcastInstanceMessage_ConsistentProtocolMessage:
			consistentMessage := protocolMessage.ConsistentProtocolMessage
			value := consistentMessage.Value

			senderPid := utils.MakeCustomPid(context.Sender())

			p.process.logger.OnMessageReceived(senderPid, message.Stamp)

			switch consistentMessage.Stage {
			case messages.ConsistentProtocolMessage_ECHO:
				p.process.verify(context, senderPid, bInstance, value)
			case messages.ConsistentProtocolMessage_VERIFY:
				if bInstance.Author == p.process.pid {
					p.process.sendMessage(
						context,
						p.process.actorPids[senderPid],
						bInstance,
						&messages.ConsistentProtocolMessage{
							Stage: messages.ConsistentProtocolMessage_ECHO,
							Value: value,
						},
					)
				} else if p.process.verify(context, senderPid, bInstance, value) {
					p.process.broadcast(
						context,
						bInstance,
						&messages.ConsistentProtocolMessage{
							Stage: messages.ConsistentProtocolMessage_ECHO,
							Value: value,
						})
				}
			}
		default:
			p.process.logger.Fatal(fmt.Sprintf("Invalid protocol message type %t", protocolMessage))
		}
	}
}

func (p *FaultyProcess) Broadcast(context actor.SenderContext, value int64) {
	p.process.Broadcast(context, value)
}

func (p *FaultyProcess) FaultyBroadcast(context actor.SenderContext, value1 int64, value2 int64) {
	broadcastInstance := &messages.BroadcastInstance{
		Author:    p.process.pid,
		SeqNumber: p.process.transactionCounter,
	}
	msgState := p.process.initMessageState(broadcastInstance)

	i := 0
	for witness := range msgState.witnessSet {
		currValue := value1
		if i >= len(msgState.witnessSet)/2 {
			currValue = value2
		}
		p.process.sendMessage(
			context,
			p.process.actorPids[witness],
			broadcastInstance,
			&messages.ConsistentProtocolMessage{
				Stage: messages.ConsistentProtocolMessage_VERIFY,
				Value: currValue,
			},
		)
		i++
	}

	p.process.transactionCounter++
}
