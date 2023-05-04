package parameters

type Parameters struct {
	// Broadcast
	ProcessCount    int `json:"n"`
	FaultyProcesses int `json:"f"`

	// Accountability
	MinOwnWitnessSetSize int `json:"w"`
	MinPotWitnessSetSize int `json:"v"`

	OwnWitnessSetRadius float64 `json:"wr"`
	PotWitnessSetRadius float64 `json:"vr"`

	WitnessThreshold        int `json:"u"`
	RecoverySwitchTimeoutNs int `json:"recovery_timeout"`
	NodeIdSize              int `json:"node_id_size"`
	NumberOfBins            int `json:"number_of_bins"`

	// Scalable reliable broadcast
	GossipSampleSize int `json:"g_size"`

	EchoSampleSize int `json:"e_size"`
	EchoThreshold  int `json:"e_threshold"`

	ReadySampleSize int `json:"r_size"`
	ReadyThreshold  int `json:"r_threshold"`

	DeliverySampleSize int `json:"d_size"`
	DeliveryThreshold  int `json:"d_threshold"`

	CleanUpTimeout int `json:"clean_up_timeout"`

	RetransmissionTimeoutNs int `json:"retransmission_timeout_ns"`
}
