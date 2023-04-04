package messages

import "fmt"

func (s *SourceMessage) Copy() *SourceMessage {
	return &SourceMessage{
		Value:     s.Value,
		Author:    s.Author,
		SeqNumber: s.SeqNumber,
	}
}

func (s *SourceMessage) ToId() string {
	return fmt.Sprintf("{%s;%d}", s.Author, s.SeqNumber)
}

func (s *SourceMessage) ToString() string {
	return fmt.Sprintf("{%s;%d;%d}", s.Author, s.SeqNumber, s.Value)
}

func (m *BrachaProtocolMessage) Copy() *BrachaProtocolMessage {
	return &BrachaProtocolMessage{
		Stage:         m.Stage,
		SourceMessage: m.SourceMessage.Copy(),
		Stamp:         m.Stamp,
	}
}

func (m *ConsistentProtocolMessage) Copy() *ConsistentProtocolMessage {
	return &ConsistentProtocolMessage{
		Stage:         m.Stage,
		SourceMessage: m.SourceMessage.Copy(),
		Stamp:         m.Stamp,
	}
}

func (m *ReliableProtocolMessage) Copy() *ReliableProtocolMessage {
	return &ReliableProtocolMessage{
		Stage:         m.Stage,
		SourceMessage: m.SourceMessage.Copy(),
		Stamp:         m.Stamp,
	}
}

func (m *RecoveryProtocolMessage) Copy() *RecoveryProtocolMessage {
	return &RecoveryProtocolMessage{
		RecoveryStage: m.RecoveryStage,
		Message:       m.Message.Copy(),
		Stamp:         m.Stamp,
	}
}

func (m *ScalableProtocolMessage) Copy() *ScalableProtocolMessage {
	return &ScalableProtocolMessage{
		Stage:         m.Stage,
		SourceMessage: m.SourceMessage.Copy(),
		Stamp:         m.Stamp,
	}
}
