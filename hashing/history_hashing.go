package hashing

type HistoryHash struct {
	binNum uint
	binCapacity uint
	hasher Hasher
	bins *MultiRing
}

func NewHistoryHash(binNum uint, binCapacity uint, hasher Hasher) *HistoryHash {
	hh := new(HistoryHash)
	hh.binNum = binNum
	hh.binCapacity = binCapacity
	hh.hasher = hasher
	hh.bins = NewMultiRing(binCapacity, binNum)
	return hh
}

func (hh *HistoryHash) Insert(bytes []byte) {
	h := toUint64(hh.hasher.Hash(bytes))
	binIndex := h % uint64(hh.binNum)
	direction := 1
	if (h & uint64(hh.binNum)) == 0 {
		direction = -1
	}
	hh.bins.add(binIndex, direction)
}
