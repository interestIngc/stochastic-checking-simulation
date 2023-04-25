package hashing

import (
	"log"
	"math/rand"
	"sort"
	"stochastic-checking-simulation/impl/utils"
	"strconv"
)

type WitnessesSelector struct {
	Hasher Hasher

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

func (ws *WitnessesSelector) GetWitnessSet(
	nodeIds []string,
	authorIndex int64, seqNumber int64, historyHash *HistoryHash,
) (map[string]bool, map[string]bool) {
	//transaction := utils.TransactionToBytes(author, seqNumber)
	//transactionRing := multiRingFromBytes(
	//	256, historyHash.binNum, ws.Hasher.Hash(transaction))

	distances := make([]dist, len(nodeIds))
	for i, pid := range nodeIds {
		historyHashRingCopy := historyHash.bins.copy()
		pidHash := ws.Hasher.Hash([]byte(pid + nodeIds[authorIndex] + strconv.Itoa(int(seqNumber))))
		idRing := multiRingFromBytes(historyHash.binCapacity, historyHash.binNum, pidHash)
		rd := rand.New(rand.NewSource(int64(utils.ToUint64(pidHash))))
		rd.Shuffle(
			int(historyHashRingCopy.dimension),
			func(i, j int) {
				historyHashRingCopy.vector[i], historyHashRingCopy.vector[j] =
					historyHashRingCopy.vector[j], historyHashRingCopy.vector[i]
			})

		idRing.merge(historyHashRingCopy)
		defaultRing := NewMultiRing(historyHash.binCapacity, historyHash.binNum)
		d, e := multiRingDistance(defaultRing, idRing, 1.0)
		//d, e := multiRingDistanceLInf(defaultRing, idRing)

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
