package consistent

import (
	"github.com/asynkron/protoactor-go/actor"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
)

type FaultyProcess struct {
	process *CorrectProcess
}

func (p *FaultyProcess) InitProcess(currPid *actor.PID, pids []*actor.PID, parameters *config.Parameters) {
	p.process = &CorrectProcess{}
	p.process.InitProcess(currPid, pids, parameters)
}

func (p *FaultyProcess) Receive(context actor.Context) {
	message := context.Message()
	switch message.(type) {
	case *messages.FaultyBroadcast:
		msg := message.(*messages.FaultyBroadcast)
		p.FaultyBroadcast(context, msg.Value1, msg.Value2)
	case *messages.ConsistentProtocolMessage:
		msg := message.(*messages.ConsistentProtocolMessage)
		msgData := msg.GetMessageData()
		sender := context.Sender()

		switch msg.Stage {
		case messages.ConsistentProtocolMessage_ECHO:
			p.process.verify(context, utils.MakeCustomPid(sender), msgData)
		case messages.ConsistentProtocolMessage_VERIFY:
			if msgData.Author == p.process.pid {
				context.RequestWithCustomSender(
					sender,
					messages.ConsistentProtocolMessage{
						Stage:       messages.ConsistentProtocolMessage_ECHO,
						MessageData: msgData,
					},
					p.process.actorPid)
			} else if p.process.verify(context, utils.MakeCustomPid(sender), msgData) {
				p.process.broadcast(
					context,
					&messages.ConsistentProtocolMessage{
						Stage:       messages.ConsistentProtocolMessage_ECHO,
						MessageData: msgData,
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
		&messages.MessageData{
			Author:    p.process.pid,
			SeqNumber: p.process.msgCounter,
			Value:     value1,
		})

	i := 0
	for witness := range msgState.witnessSet {
		currValue := value1
		if i >= len(msgState.witnessSet)/2 {
			currValue = value2
		}

		context.RequestWithCustomSender(
			p.process.actorPids[witness],
			&messages.ConsistentProtocolMessage{
				Stage: messages.ConsistentProtocolMessage_VERIFY,
				MessageData: &messages.MessageData{
					Author:    p.process.pid,
					SeqNumber: p.process.msgCounter,
					Value:     currValue,
				},
			},
			p.process.actorPid)

		i++
	}

	p.process.msgCounter++
}
