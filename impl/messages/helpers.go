package messages

func (s *SourceMessage) Copy() *SourceMessage {
	return &SourceMessage{
		Value:     s.Value,
		Author:    s.Author,
		SeqNumber: s.SeqNumber,
	}
}

func (m *BrachaProtocolMessage) Copy() *BrachaProtocolMessage {
	return &BrachaProtocolMessage{
		Stage:         m.Stage,
		SourceMessage: m.SourceMessage.Copy(),
	}
}

func (m *ConsistentProtocolMessage) Copy() *ConsistentProtocolMessage {
	return &ConsistentProtocolMessage{
		Stage:         m.Stage,
		SourceMessage: m.SourceMessage.Copy(),
	}
}

func (m *ReliableProtocolMessage) Copy() *ReliableProtocolMessage {
	return &ReliableProtocolMessage{
		Stage:         m.Stage,
		SourceMessage: m.SourceMessage.Copy(),
	}
}

func (m *RecoveryProtocolMessage) Copy() *RecoveryProtocolMessage {
	return &RecoveryProtocolMessage{
		RecoveryStage: m.RecoveryStage,
		Message:       m.Message.Copy(),
	}
}

func (m *ScalableProtocolMessage) Copy() *ScalableProtocolMessage {
	return &ScalableProtocolMessage{
		Stage:         m.Stage,
		SourceMessage: m.SourceMessage.Copy(),
	}
}
