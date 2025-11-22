package buffer

import (
	"errors"
	"io"

	"github.com/ninepeach/ark/alloc"
)

// DefaultSize is the default buffer size used by New().
const DefaultSize = 32 * 1024

// Buffer is a simple growable byte buffer with read/write indexes.
// It uses alloc.Get/Put for underlying storage when possible.
type Buffer struct {
	data   []byte
	start  int // read index
	end    int // write index (exclusive)
	pooled bool
}

// New creates a buffer with DefaultSize capacity.
func New() *Buffer {
	return NewSize(DefaultSize)
}

// NewSize creates a buffer with an initial capacity of size.
// It uses alloc.Get(size) when possible; if alloc returns nil,
// it falls back to make([]byte, size).
func NewSize(size int) *Buffer {
	if size < 0 {
		size = 0
	}
	b := &Buffer{}
	if size == 0 {
		b.data = nil
		b.pooled = false
		return b
	}

	data := alloc.Get(size)
	if data != nil {
		// data is pooled
		b.data = data[:size]
		b.pooled = true
		return b
	}

	// alloc.Get returned nil => fallback to direct allocation
	b.data = make([]byte, size)
	b.pooled = false
	return b
}

// FromBytes wraps an existing byte slice as a Buffer (readable content = full slice).
// It does not copy the data and does not use the pool.
func FromBytes(b []byte) *Buffer {
	return &Buffer{
		data:   b,
		start:  0,
		end:    len(b),
		pooled: false,
	}
}

// Bytes returns the current readable slice.
func (b *Buffer) Bytes() []byte {
	return b.data[b.start:b.end]
}

// Len returns the number of readable bytes.
func (b *Buffer) Len() int {
	return b.end - b.start
}

// Cap returns the total capacity of the underlying slice.
func (b *Buffer) Cap() int {
	return len(b.data)
}

// IsEmpty reports whether there is no readable data.
func (b *Buffer) IsEmpty() bool {
	return b.Len() == 0
}

// Reset clears the buffer content but keeps the underlying slice.
func (b *Buffer) Reset() {
	b.start = 0
	b.end = 0
}

// grow ensures there is at least n more bytes of free space for writing.
func (b *Buffer) grow(n int) {
	if n <= 0 {
		return
	}
	free := len(b.data) - b.end
	if free >= n {
		return
	}

	// Try to compact first (move unread data to the beginning).
	if b.start > 0 && b.Len() > 0 {
		copy(b.data, b.data[b.start:b.end])
		b.end = b.Len()
		b.start = 0
		free = len(b.data) - b.end
		if free >= n {
			return
		}
	}

	// Need to allocate a bigger slice.
	curLen := b.Len()
	newCap := curLen + n
	if newCap < len(b.data)*2 {
		newCap = len(b.data) * 2
		if newCap == 0 {
			newCap = n
		}
	}

	newData := make([]byte, newCap)
	if curLen > 0 {
		copy(newData, b.data[b.start:b.end])
	}
	b.data = newData
	b.start = 0
	b.end = curLen
	// The new slice is not from pool.
	b.pooled = false
}

// Extend reserves n bytes at the end and returns the slice for caller to fill.
func (b *Buffer) Extend(n int) []byte {
	if n < 0 {
		panic("buffer: negative extend size")
	}
	b.grow(n)
	start := b.end
	b.end += n
	return b.data[start:b.end]
}

// Write appends data to the buffer.
func (b *Buffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	b.grow(len(p))
	n := copy(b.data[b.end:], p)
	b.end += n
	return n, nil
}

// WriteByte appends a single byte to the buffer.
func (b *Buffer) WriteByte(c byte) error {
	b.grow(1)
	b.data[b.end] = c
	b.end++
	return nil
}

// Read reads from the buffer into p.
func (b *Buffer) Read(p []byte) (int, error) {
	if b.IsEmpty() {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.start:b.end])
	b.start += n
	if b.start == b.end {
		// All consumed, reset indexes.
		b.start = 0
		b.end = 0
	}
	return n, nil
}

// ReadByte reads and returns a single byte.
func (b *Buffer) ReadByte() (byte, error) {
	if b.IsEmpty() {
		return 0, io.EOF
	}
	c := b.data[b.start]
	b.start++
	if b.start == b.end {
		b.start = 0
		b.end = 0
	}
	return c, nil
}

// To returns the first n bytes of the readable region.
// If n > Len(), it clamps to Len().
func (b *Buffer) To(n int) []byte {
	if n <= 0 {
		return nil
	}
	if n > b.Len() {
		n = b.Len()
	}
	return b.data[b.start : b.start+n]
}

// ReadBytes returns exactly n bytes (or error if not enough).
func (b *Buffer) ReadBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, errors.New("buffer: negative length")
	}
	if b.Len() < n {
		return nil, io.EOF
	}
	out := make([]byte, n)
	copy(out, b.data[b.start:b.start+n])
	b.start += n
	if b.start == b.end {
		b.start = 0
		b.end = 0
	}
	return out, nil
}

// Release returns the underlying slice to the alloc pool if it came from there,
// and resets the Buffer to zero value.
func (b *Buffer) Release() {
	if b == nil {
		return
	}
	if b.pooled && b.data != nil {
		_ = alloc.Put(b.data)
	}
	*b = Buffer{}
}
