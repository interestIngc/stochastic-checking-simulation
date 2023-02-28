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

func (el *EventLogger) Printf(message string, args ...any) {
	el.logger.Printf(message, args)
}

func (el *EventLogger) LogAccept(msgData *messages.MessageData, messagesReceived int) {
	el.logger.Printf(
		"%s: Accepted transaction with seq number %d and value %d from %s, messages received: %d\n",
		el.pid, msgData.SeqNumber, msgData.Value, msgData.Author, messagesReceived)
}

func (el *EventLogger) LogHistoryHash(historyHash *hashing.HistoryHash) {
	el.logger.Printf("%s: History hash is %s\n", el.pid, historyHash.ToString())
}

func (el *EventLogger) LogMessageLatency(from string, timestamp int64) {
	el.logger.Printf(
		"%s: received message from %s, latency %d",
		el.pid, from, utils.GetNow()-timestamp)
}

func (el *EventLogger) LogAttack() {
	el.logger.Printf("%s: Detected a duplicated seq number attack\n", el.pid)
}
