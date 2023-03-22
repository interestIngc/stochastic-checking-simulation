package eventlogger

import (
	"log"
	"stochastic-checking-simulation/impl/hashing"
	"stochastic-checking-simulation/impl/messages"
	"stochastic-checking-simulation/impl/utils"
)

type EventLogger struct {
	pid    string
	logger *log.Logger
}

func InitEventLogger(pid string, logger *log.Logger) *EventLogger {
	l := new(EventLogger)
	l.pid = pid
	l.logger = logger
	return l
}

func (el *EventLogger) OnTransactionInit(seqNumber int64) {
	el.logger.Printf("Initialising transaction {%s;%d}, timestamp: %d\n",
		el.pid, seqNumber, utils.GetNow())
}

func (el *EventLogger) OnAccept(msgData *messages.MessageData, messagesReceived int) {
	el.logger.Printf(
		"Accepted transaction {%s;%d}. Value: %d, messages received: %d, timestamp: %d\n",
		msgData.Author, msgData.SeqNumber, msgData.Value, messagesReceived, utils.GetNow())
}

func (el *EventLogger) OnHistoryHashUpdate(
	msgData *messages.MessageData, historyHash *hashing.HistoryHash) {
	el.logger.Printf(
		"History hash after accepting transaction {%s;%d} is %s\n",
		msgData.Author, msgData.SeqNumber, historyHash.ToString())
}

func (el *EventLogger) OnAttack(msgData *messages.MessageData, committedValue int64) {
	el.logger.Printf(
		"Detected a duplicated seq number attack. " +
			"Transaction: {%s;%d}, received value: %d, committed value: %d, timestamp: %d\n",
		msgData.Author, msgData.SeqNumber, msgData.Value, committedValue, utils.GetNow())
}

func (el *EventLogger) OnMessageSent(msgId int64) {
	el.logger.Printf("Sent message: {%s;%d}, timestamp: %d\n", el.pid, msgId, utils.GetNow())
}

func (el *EventLogger) OnMessageReceived(senderPid string, msgId int64) {
	el.logger.Printf("Received message: {%s;%d}, timestamp: %d\n", senderPid, msgId, utils.GetNow())
}
