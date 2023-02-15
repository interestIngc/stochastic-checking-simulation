package hashing

import (
	"fmt"
	"math/rand"
	"sort"
	"stochastic-checking-simulation/config"
	"stochastic-checking-simulation/impl/utils"
)

type WitnessesSelector struct {
	NodeIds []string
	Hasher  Hasher
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
	author string, seqNumber int64, historyHash *HistoryHash,
) (map[string]bool, map[string]bool) {
	transaction := utils.TransactionToBytes(author, seqNumber)
	transactionRing := multiRingFromBytes(
		256, historyHash.binNum, ws.Hasher.Hash(transaction))

	distances := make([]dist, len(ws.NodeIds))
	for i, pid := range ws.NodeIds {
		historyHashRingCopy := historyHash.bins.copy()
		pidHash := ws.Hasher.Hash([]byte(pid))
		rd := rand.New(rand.NewSource(int64(utils.ToUint64(pidHash))))
		rd.Shuffle(
			int(historyHashRingCopy.dimension),
			func(i, j int) {
				historyHashRingCopy.vector[i], historyHashRingCopy.vector[j] =
					historyHashRingCopy.vector[j], historyHashRingCopy.vector[i]
			})

		d, e := multiRingDistance(historyHashRingCopy, transactionRing, 1.0)

		if e != nil {
			fmt.Printf("Error while generating a witness set happened: %s\n", e)
			return nil, nil
		}

		distances[i] = dist{ind: i, d: d}
	}

	sort.Sort(byDist(distances))

	potWitnessSet := make(map[string]bool)
	ownWitnessSet := make(map[string]bool)

	for i := 0; i < config.PotWitnessSetSize; i++ {
		id := ws.NodeIds[distances[i].ind]
		potWitnessSet[id] = true
		if i < config.OwnWitnessSetSize {
			ownWitnessSet[id] = true
		}
	}

	//for i := 0; i < len(ws.NodeIds) && distances[i].d < config.PotWitnessSetRadius; i++ {
	//	id := ws.NodeIds[distances[i].ind]
	//	potWitnessSet[id] = true
	//	if distances[i].d < config.OwnWitnessSetRadius {
	//		ownWitnessSet[id] = true
	//	}
	//}

	return ownWitnessSet, potWitnessSet
}
