package detect

// A nil bitset is valid and holds nothing, which is what lets callers leave it
// unallocated.
type bitset []uint64

func newBitset(n int) bitset {
	return make(bitset, (n+63)/64)
}

func (b bitset) set(i int) {
	b[i/64] |= 1 << uint(i%64)
}

func (b bitset) has(i int) bool {
	if i < 0 || i/64 >= len(b) {
		return false
	}
	return b[i/64]&(1<<uint(i%64)) != 0
}
