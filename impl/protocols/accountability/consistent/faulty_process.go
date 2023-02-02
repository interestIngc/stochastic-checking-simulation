package consistent

import (
	"github.com/asynkron/protoactor-go/actor"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
)

type FaultyProcess struct {
	process *CorrectProcess
}

func (p *FaultyProcess) InitProcess(currPid *actor.PID, pids []*actor.PID) {
	p.process = &CorrectProcess{}
	p.process.InitProcess(currPid, pids)
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
		senderId := context.Sender()

		switch msg.Stage {
		case messages.ConsistentProtocolMessage_ECHO:
			p.process.verify(context, utils.PidToString(senderId), msgData)
		case messages.ConsistentProtocolMessage_VERIFY:
			if msgData.Author == utils.PidToString(p.process.currPid) {
				context.RequestWithCustomSender(
					senderId,
					messages.ConsistentProtocolMessage{
						Stage:       messages.ConsistentProtocolMessage_ECHO,
						MessageData: msgData,
					},
					p.process.currPid)
			} else if p.process.verify(context, utils.PidToString(senderId), msgData) {
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
	author := utils.PidToString(p.process.currPid)
	seqNumber := p.process.msgCounter
	p.process.msgCounter++

	msgState := newMessageState()
	msgState.witnessSet, _ =
		p.process.wSelector.GetWitnessSet(
			author,
			seqNumber,
			p.process.historyHash,
		)
	p.process.messagesLog[author][seqNumber] = msgState

	i := 0
	for witness := range msgState.witnessSet {
		if i == len(msgState.witnessSet)/2 {
			break
		}
		context.RequestWithCustomSender(
			p.process.pids[witness],
			&messages.ConsistentProtocolMessage{
				Stage: messages.ConsistentProtocolMessage_VERIFY,
				MessageData: &messages.MessageData{
					Author:    author,
					SeqNumber: seqNumber,
					Value:     value1,
				},
			},
			p.process.currPid)
		i++
	}
	for witness := range msgState.witnessSet {
		context.RequestWithCustomSender(
			p.process.pids[witness],
			&messages.ConsistentProtocolMessage{
				Stage: messages.ConsistentProtocolMessage_VERIFY,
				MessageData: &messages.MessageData{
					Author:    author,
					SeqNumber: seqNumber,
					Value:     value2,
				},
			},
			p.process.currPid)
	}
}
