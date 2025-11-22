package alloc

import (
	"math/bits"
	"math/rand"
	"testing"
)

func TestAllocatorGet(t *testing.T) {
	a := NewAllocator()

	if a.Get(0) != nil {
		t.Fatal("Get(0) should return nil")
	}

	if b := a.Get(1); len(b) != 1 || cap(b) != 1 {
		t.Fatalf("Get(1): len=%d cap=%d, want len=1 cap=1", len(b), cap(b))
	}

	if b := a.Get(2); len(b) != 2 || cap(b) != 2 {
		t.Fatalf("Get(2): len=%d cap=%d, want len=2 cap=2", len(b), cap(b))
	}

	if b := a.Get(3); len(b) != 3 || cap(b) != 4 {
		t.Fatalf("Get(3): len=%d cap=%d, want len=3 cap=4", len(b), cap(b))
	}

	if b := a.Get(4); len(b) != 4 || cap(b) != 4 {
		t.Fatalf("Get(4): len=%d cap=%d, want len=4 cap=4", len(b), cap(b))
	}

	if b := a.Get(1023); len(b) != 1023 || cap(b) != 1024 {
		t.Fatalf("Get(1023): len=%d cap=%d, want len=1023 cap=1024", len(b), cap(b))
	}

	if b := a.Get(1024); len(b) != 1024 || cap(b) != 1024 {
		t.Fatalf("Get(1024): len=%d cap=%d, want len=1024 cap=1024", len(b), cap(b))
	}

	if b := a.Get(65536); len(b) != 65536 || cap(b) != 65536 {
		t.Fatalf("Get(65536): len=%d cap=%d, want len=65536 cap=65536", len(b), cap(b))
	}

	if a.Get(65537) != nil {
		t.Fatal("Get(65537) should return nil")
	}
}

func TestAllocatorPut(t *testing.T) {
	a := NewAllocator()

	if err := a.Put(nil); err == nil {
		t.Fatal("Put(nil) should return error")
	}

	// cap not power of two
	if err := a.Put(make([]byte, 3)); err == nil {
		t.Fatal("Put(cap=3) should return error")
	}

	// cap = 4 is valid
	if err := a.Put(make([]byte, 4)); err != nil {
		t.Fatalf("Put(cap=4) error: %v", err)
	}

	// cap = 65536 is valid
	if err := a.Put(make([]byte, 65536)); err != nil {
		t.Fatalf("Put(cap=65536) error: %v", err)
	}

	// too large
	if err := a.Put(make([]byte, 65537)); err == nil {
		t.Fatal("Put(cap=65537) should return error")
	}
}

func TestAllocatorReuse(t *testing.T) {
	a := NewAllocator()

	b := a.Get(4)
	capB := cap(b)

	if err := a.Put(b); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	b2 := a.Get(4)
	if cap(b2) != capB {
		t.Fatalf("allocator did not reuse pool: cap1=%d cap2=%d", capB, cap(b2))
	}
}

func BenchmarkMSB(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = bits.Len(uint(rand.Intn(MaxSize) + 1))
	}
}

func BenchmarkAllocator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		size := (i % MaxSize) + 1
		buf := defaultAllocator.Get(size)
		if buf != nil {
			_ = defaultAllocator.Put(buf)
		}
	}
}
