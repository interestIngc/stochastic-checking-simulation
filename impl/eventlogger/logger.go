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

func (el *EventLogger) OnDeliver(
	sourceMessage *messages.SourceMessage, messagesReceived int) {
	el.logger.Printf(
		"Delivered transaction {%s;%d}. Value: %d, messages received: %d, timestamp: %d\n",
		sourceMessage.Author,
		sourceMessage.SeqNumber,
		sourceMessage.Value,
		messagesReceived,
		utils.GetNow())
}

func (el *EventLogger) OnHistoryHashUpdate(
	sourceMessage *messages.SourceMessage, historyHash *hashing.HistoryHash) {
	el.logger.Printf(
		"History hash after delivering transaction {%s;%d} is %s\n",
		sourceMessage.Author, sourceMessage.SeqNumber, historyHash.ToString())
}

func (el *EventLogger) OnAttack(
	sourceMessage *messages.SourceMessage, committedValue int64) {
	el.logger.Printf(
		"Detected a duplicated seq number attack. "+
			"Transaction: {%s;%d}, received value: %d, committed value: %d, timestamp: %d\n",
		sourceMessage.Author,
		sourceMessage.SeqNumber,
		sourceMessage.Value,
		committedValue,
		utils.GetNow())
}

func (el *EventLogger) OnMessageSent(msgId int64) {
	el.logger.Printf(
		"Sent message: {%s;%d}, timestamp: %d\n",
		el.pid, msgId, utils.GetNow())
}

func (el *EventLogger) OnMessageReceived(senderPid string, msgId int64) {
	el.logger.Printf(
		"Received message: {%s;%d}, timestamp: %d\n",
		senderPid, msgId, utils.GetNow())
}
