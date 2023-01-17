package hashing

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"math/rand"
	"sort"
	"stochastic-checking-simulation/config"
)

type WitnessesSelector struct {
	NodeIds []*actor.PID
	Hasher  Hasher
}

type dist struct {
	ind int
	d float64
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
		author *actor.PID, seqNumber int32, historyHash *HistoryHash,
	) []*actor.PID {
	transaction := TransactionToBytes(author, seqNumber)
	transactionRing := multiRingFromBytes(256, historyHash.binNum, ws.Hasher.Hash(transaction))

	distances := make([]dist, len(ws.NodeIds))
	for i, pid := range ws.NodeIds {
		historyHashRingCopy := historyHash.bins.copy()
		pidHash := ws.Hasher.Hash([]byte(pid.String()))
		rd := rand.New(rand.NewSource(int64(toUint64(pidHash))))
		rd.Shuffle(
			int(historyHashRingCopy.dimension),
			func(i, j int) {
				historyHashRingCopy.vector[i], historyHashRingCopy.vector[j] =
					historyHashRingCopy.vector[j], historyHashRingCopy.vector[i]
			})

		d, e := multiRingDistance(historyHashRingCopy, transactionRing, 1.0)

		if e != nil {
			fmt.Printf("Error while generating a witness set happened: %s\n", e)
			return nil
		}

		distances[i] = dist{ind: i, d: d}
	}

	sort.Sort(byDist(distances))

	witnessSet := make([]*actor.PID, config.WitnessSetSize)
	for i := 0; i < config.WitnessSetSize; i++ {
		witnessSet[i] = ws.NodeIds[distances[i].ind]
	}
	return witnessSet
}
