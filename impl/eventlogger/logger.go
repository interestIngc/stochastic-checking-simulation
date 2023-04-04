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

func (el *EventLogger) OnTransactionInit(sourceMessage *messages.SourceMessage) {
	el.logger.Printf(
		"Initialising transaction: %s, timestamp: %d\n",
		sourceMessage.ToId(), utils.GetNow())
}

func (el *EventLogger) OnWitnessSetSelected(
	wsType string,
	sourceMessage *messages.SourceMessage,
	ws map[string]bool,
) {
	pids := make([]string, len(ws))
	i := 0
	for pid := range ws {
		pids[i] = pid
		i++
	}

	el.logger.Printf(
		"Witness set selected; type: %s, transaction: %s, pids: %v\n",
		wsType, sourceMessage.ToId(), pids)
}

func (el *EventLogger) OnRecoveryProtocolSwitch(sourceMessage *messages.SourceMessage) {
	el.logger.Printf(
		"Switching to the recovery protocol; transaction: %s, timestamp: %d\n",
		sourceMessage.ToId(), utils.GetNow())
}

func (el *EventLogger) OnDeliver(
	sourceMessage *messages.SourceMessage, messagesReceived int) {
	el.logger.Printf(
		"Delivered transaction: %s, value: %d, messages received: %d, timestamp: %d\n",
		sourceMessage.ToId(),
		sourceMessage.Value,
		messagesReceived,
		utils.GetNow())
}

func (el *EventLogger) OnHistoryUsedInWitnessSetSelection(
	sourceMessage *messages.SourceMessage,
	historyHash *hashing.HistoryHash,
	deliveredMessagesHistory []string,
) {
	el.logger.Printf(
		"Witness set selection; transaction: %s, history hash: %s, history: %v\n",
		sourceMessage.ToId(), historyHash.ToString(), deliveredMessagesHistory)
}

func (el *EventLogger) OnAttack(
	sourceMessage *messages.SourceMessage, committedValue int64) {
	el.logger.Printf(
		"Detected a duplicated seq number attack; "+
			"transaction: %s, received value: %d, committed value: %d, timestamp: %d\n",
		sourceMessage.ToId(),
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
