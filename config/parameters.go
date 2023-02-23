package config

import "time"

type Parameters struct {
	// Broadcast
	ProcessCount    int
	FaultyProcesses int

	// Accountability
	MinOwnWitnessSetSize int
	MinPotWitnessSetSize int

	OwnWitnessSetRadius float64
	PotWitnessSetRadius float64

	WitnessThreshold        int
	RecoverySwitchTimeoutNs time.Duration
	NodeIdSize              int
	NumberOfBins            int

	// Scalable reliable broadcast
	GossipSampleSize int

	EchoSampleSize int
	EchoThreshold  int

	ReadySampleSize int
	ReadyThreshold  int

	DeliverySampleSize int
	DeliveryThreshold  int
}
