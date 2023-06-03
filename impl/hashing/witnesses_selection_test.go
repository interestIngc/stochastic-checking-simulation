package hashing

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var nodeIds = []string{"0", "1", "2"}

const authorIndex = 0
const seqNumber = 2

type defaultHasher struct{}

func (ws *defaultHasher) Hash(bytes []byte) []byte {
	return bytes
}

func TestGetWitnessSet_allWitnessesReturned(t *testing.T) {
	hasher := &defaultHasher{}
	ws := WitnessesSelector{
		Hasher:               hasher,
		MinPotWitnessSetSize: 3,
		MinOwnWitnessSetSize: 3,
		PotWitnessSetRadius:  0,
		OwnWitnessSetRadius:  0,
	}
	historyHash := NewHistoryHash(3, 8, hasher)

	ownWitnessSet, potWitnessSet :=
		ws.GetWitnessSet(nodeIds, authorIndex, seqNumber, historyHash)

	expectedWitnessSet := map[string]bool{"0": true, "1": true, "2": true}
	assert.Exactly(t, expectedWitnessSet, ownWitnessSet)
	assert.Exactly(t, expectedWitnessSet, potWitnessSet)
}

func TestGetWitnessSet_twoClosestWitnessesReturned(t *testing.T) {
	hasher := &defaultHasher{}
	ws := WitnessesSelector{
		Hasher:               hasher,
		MinPotWitnessSetSize: 3,
		MinOwnWitnessSetSize: 2,
		PotWitnessSetRadius:  0,
		OwnWitnessSetRadius:  0,
	}
	historyHash := NewHistoryHash(3, 8, hasher)

	ownWitnessSet, potWitnessSet :=
		ws.GetWitnessSet(nodeIds, authorIndex, seqNumber, historyHash)

	expectedOwnWitnessSet := map[string]bool{"0": true, "1": true}
	expectedPotWitnessSet := map[string]bool{"0": true, "1": true, "2": true}
	assert.Exactly(t, expectedOwnWitnessSet, ownWitnessSet)
	assert.Exactly(t, expectedPotWitnessSet, potWitnessSet)
}

func TestGetWitnessSet_allWitnessesInRadiusReturned(t *testing.T) {
	hasher := &defaultHasher{}
	ws := WitnessesSelector{
		Hasher:               hasher,
		MinPotWitnessSetSize: 3,
		MinOwnWitnessSetSize: 2,
		PotWitnessSetRadius:  10.0,
		OwnWitnessSetRadius:  10.0,
	}
	historyHash := NewHistoryHash(3, 8, hasher)

	ownWitnessSet, potWitnessSet :=
		ws.GetWitnessSet(nodeIds, authorIndex, seqNumber, historyHash)

	expectedWitnessSet := map[string]bool{"0": true, "1": true, "2": true}
	assert.Exactly(t, expectedWitnessSet, ownWitnessSet)
	assert.Exactly(t, expectedWitnessSet, potWitnessSet)
}
