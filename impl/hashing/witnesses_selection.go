package hashing

import (
	"log"
	"sort"
)

// WitnessesSelector enables witness set selection.
type WitnessesSelector struct {
	MinPotWitnessSetSize int
	MinOwnWitnessSetSize int
	PotWitnessSetRadius  float64
	OwnWitnessSetRadius  float64
}

type dist struct {
	ind int
	d   float64
}

type byDist []dist

func (bd byDist) Len() int {
	return len(bd)
}

func (bd byDist) Swap(i, j int) {
	bd[i], bd[j] = bd[j], bd[i]
}

func (bd byDist) Less(i, j int) bool {
	return bd[i].d < bd[j].d
}

// GetWitnessSet returns own and potential witness sets for a given transaction,
// which is identified by a pair of authorId and seqNumber, and a given history hash.
func (ws *WitnessesSelector) GetWitnessSet(
	nodeIds []string,
	historyHashes []*HistoryHash,
) (map[string]bool, map[string]bool) {
	distances := make([]dist, len(nodeIds))
	for i := range nodeIds {
		defaultRing := NewMultiRing(historyHashes[0].binCapacity, historyHashes[0].binNum)
		d, e := multiRingDistance(defaultRing, historyHashes[i].bins)

		if e != nil {
			log.Printf("Error while generating a witness set happened: %s\n", e)
			return nil, nil
		}

		distances[i] = dist{ind: i, d: d}
	}

	sort.Sort(byDist(distances))

	potWitnessSet := make(map[string]bool)
	ownWitnessSet := make(map[string]bool)

	for i := 0; i < len(nodeIds) &&
		(i < ws.MinPotWitnessSetSize || distances[i].d < ws.PotWitnessSetRadius); i++ {
		id := nodeIds[distances[i].ind]
		potWitnessSet[id] = true
		if i < ws.MinOwnWitnessSetSize || distances[i].d < ws.OwnWitnessSetRadius {
			ownWitnessSet[id] = true
		}
	}

	return ownWitnessSet, potWitnessSet
}
