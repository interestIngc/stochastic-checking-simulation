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

func (el *EventLogger) OnTransactionInit(
	broadcastInstance *messages.BroadcastInstance,
) {
	el.logger.Printf(
		"Initialising transaction: %s, timestamp: %d\n",
		broadcastInstance.ToString(), utils.GetNow())
}

func (el *EventLogger) OnWitnessSetSelected(
	wsType string,
	broadcastInstance *messages.BroadcastInstance,
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
		wsType, broadcastInstance.ToString(), pids)
}

func (el *EventLogger) OnRecoveryProtocolSwitch(broadcastInstance *messages.BroadcastInstance) {
	el.logger.Printf(
		"Switching to the recovery protocol; transaction: %s, timestamp: %d\n",
		broadcastInstance.ToString(), utils.GetNow())
}

func (el *EventLogger) OnDeliver(
	broadcastInstance *messages.BroadcastInstance, value int64, messagesReceived int) {
	el.logger.Printf(
		"Delivered transaction: %s, value: %d, messages received: %d, timestamp: %d\n",
		broadcastInstance.ToString(),
		value,
		messagesReceived,
		utils.GetNow())
}

func (el *EventLogger) OnHistoryUsedInWitnessSetSelection(
	broadcastInstance *messages.BroadcastInstance,
	historyHash *hashing.HistoryHash,
	deliveredMessagesHistory []string,
) {
	el.logger.Printf(
		"Witness set selection; transaction: %s, history hash: %s, history: %v\n",
		broadcastInstance.ToString(), historyHash.ToString(), deliveredMessagesHistory)
}

func (el *EventLogger) OnAttack(
	broadcastInstance *messages.BroadcastInstance,
	receivedValue int64,
	committedValue int64,
) {
	el.logger.Printf(
		"Detected a duplicated seq number attack; "+
			"transaction: %s, received value: %d, committed value: %d, timestamp: %d\n",
		broadcastInstance.ToString(),
		receivedValue,
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

func (el *EventLogger) Fatal(message string) {
	el.logger.Fatal(message)
}
