package config

import "time"

// Broadcast

var ProcessCount = 256
var FaultyProcesses = 20

// Accountability

var MinOwnWitnessSetSize = 16
var MinPotWitnessSetSize = 32

var OwnWitnessSetRadius = 1900.0
var PotWitnessSetRadius = 1910.0

var WitnessThreshold = 4
var RecoverySwitchTimeoutNs time.Duration = 1
var NodeIdSize = 256
var NumberOfBins = 32

// Scalable reliable broadcast

var GossipSampleSize = 20

var EchoSampleSize = 16
var EchoThreshold = 10

var ReadySampleSize = 20
var ReadyThreshold = 10

var DeliverySampleSize = 20
var DeliveryThreshold = 15
