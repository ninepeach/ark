package alloc

import (
	"errors"
	"math/bits"
	"sync"
)

// MaxSize is the maximum supported buffer size (64KiB).
const MaxSize = 65536

// Allocator manages a set of power-of-two sized byte slice pools.
//
// Pool index i holds buffers of size 1<<i, for i in [0, 16], i.e. 1B..64KiB.
type Allocator struct {
	buffers []sync.Pool
}

// defaultAllocator is the package-level allocator used by Get/Put.
var defaultAllocator = NewAllocator()

// NewAllocator creates a new Allocator with pools for 1B..64KiB.
func NewAllocator() *Allocator {
	const maxBits = 16 // 2^16 = 65536

	a := &Allocator{
		buffers: make([]sync.Pool, maxBits+1),
	}

	for i := range a.buffers {
		size := 1 << uint(i)
		a.buffers[i].New = func() any {
			// allocate a slice of the exact power-of-two size
			return make([]byte, size)
		}
	}

	return a
}

// msb returns floor(log2(size)) for size > 0.
// For example: msb(1)=0, msb(2)=1, msb(3)=1, msb(4)=2.
func msb(size int) int {
	if size <= 0 {
		return 0
	}
	return bits.Len(uint(size)) - 1
}

// Get returns a byte slice with length == size and capacity being
// the smallest power of two >= size, with an upper bound of MaxSize.
// If size <= 0 or size > MaxSize, it returns nil.
func (a *Allocator) Get(size int) []byte {
	if size <= 0 || size > MaxSize {
		return nil
	}

	idx := msb(size)
	if size != 1<<idx {
		idx++
	}
	if idx < 0 || idx >= len(a.buffers) {
		return nil
	}

	buf := a.buffers[idx].Get().([]byte)
	// shrink length to requested size but keep capacity (power of two)
	return buf[:size]
}

// Put returns a buffer to the allocator.
//
// The capacity of buf must be a power of two and <= MaxSize.
// Otherwise, Put returns an error and does not store the buffer.
func (a *Allocator) Put(buf []byte) error {
	if buf == nil {
		return errors.New("alloc: Put(nil)")
	}
	c := cap(buf)
	if c <= 0 || c > MaxSize {
		return errors.New("alloc: Put() incorrect buffer size")
	}
	// capacity must be power of two
	if c&(c-1) != 0 {
		return errors.New("alloc: Put() incorrect buffer size (not power of two)")
	}

	idx := msb(c)
	if idx < 0 || idx >= len(a.buffers) {
		return errors.New("alloc: Put() invalid pool index")
	}

	// Reset length to full capacity before putting back.
	buf = buf[:c]
	a.buffers[idx].Put(buf)
	return nil
}

// Get is a convenience wrapper around the package-level default allocator.
func Get(size int) []byte {
	return defaultAllocator.Get(size)
}

// Put returns a buffer to the package-level default allocator.
func Put(buf []byte) error {
	return defaultAllocator.Put(buf)
}
