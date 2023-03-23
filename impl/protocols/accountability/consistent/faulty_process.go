package consistent

import (
	"github.com/asynkron/protoactor-go/actor"
	"log"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/parameters"
	"stochastic-checking-simulation/impl/utils"
)

type FaultyProcess struct {
	process *CorrectProcess
}

func (p *FaultyProcess) InitProcess(
	currPid *actor.PID,
	pids []*actor.PID,
	parameters *parameters.Parameters,
	logger *log.Logger) {
	p.process = &CorrectProcess{}
	p.process.InitProcess(currPid, pids, parameters, logger)
}

func (p *FaultyProcess) Receive(context actor.Context) {
	message := context.Message()
	switch message.(type) {
	case *messages.FaultyBroadcast:
		msg := message.(*messages.FaultyBroadcast)
		p.FaultyBroadcast(context, msg.Value1, msg.Value2)
	case *messages.ConsistentProtocolMessage:
		msg := message.(*messages.ConsistentProtocolMessage)
		sourceMessage := msg.SourceMessage
		sender := context.Sender()
		senderPid := utils.MakeCustomPid(sender)

		p.process.logger.OnMessageReceived(senderPid, msg.Stamp)

		switch msg.Stage {
		case messages.ConsistentProtocolMessage_ECHO:
			p.process.verify(context, senderPid, sourceMessage)
		case messages.ConsistentProtocolMessage_VERIFY:
			if sourceMessage.Author == p.process.pid {
				p.process.sendMessage(
					context,
					sender,
					&messages.ConsistentProtocolMessage{
						Stage:         messages.ConsistentProtocolMessage_ECHO,
						SourceMessage: sourceMessage,
					},
				)
			} else if p.process.verify(context, senderPid, sourceMessage) {
				p.process.broadcast(
					context,
					&messages.ConsistentProtocolMessage{
						Stage:         messages.ConsistentProtocolMessage_ECHO,
						SourceMessage: sourceMessage,
					})
			}
		}
	}
}

func (p *FaultyProcess) Broadcast(context actor.SenderContext, value int64) {
	p.process.Broadcast(context, value)
}

func (p *FaultyProcess) FaultyBroadcast(context actor.SenderContext, value1 int64, value2 int64) {
	msgState := p.process.initMessageState(
		&messages.SourceMessage{
			Author:    p.process.pid,
			SeqNumber: p.process.transactionCounter,
			Value:     value1,
		})

	i := 0
	for witness := range msgState.witnessSet {
		currValue := value1
		if i >= len(msgState.witnessSet)/2 {
			currValue = value2
		}
		p.process.sendMessage(
			context,
			p.process.actorPids[witness],
			&messages.ConsistentProtocolMessage{
				Stage: messages.ConsistentProtocolMessage_VERIFY,
				SourceMessage: &messages.SourceMessage{
					Author:    p.process.pid,
					SeqNumber: p.process.transactionCounter,
					Value:     currValue,
				},
			},
		)
		i++
	}

	p.process.transactionCounter++
}
