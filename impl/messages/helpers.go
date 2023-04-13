package messages

import "fmt"

func (b *BroadcastInstance) ToString() string {
	return fmt.Sprintf("{%s;%d}", b.Author, b.SeqNumber)
}

func (b *BroadcastInstance) Copy() *BroadcastInstance {
	return &BroadcastInstance{
		Author:    b.Author,
		SeqNumber: b.SeqNumber,
	}
}

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
		Stage: m.Stage,
		Value: m.Value,
	}
}

func (m *RecoveryProtocolMessage) Copy() *RecoveryProtocolMessage {
	return &RecoveryProtocolMessage{
		Stage:   m.Stage,
		ReliableProtocolMessage: m.ReliableProtocolMessage,
	}
}

func (m *ScalableProtocolMessage) Copy() *ScalableProtocolMessage {
	return &ScalableProtocolMessage{
		Stage:         m.Stage,
		SourceMessage: m.SourceMessage.Copy(),
		Stamp:         m.Stamp,
	}
}
