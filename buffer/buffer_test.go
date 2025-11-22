package buffer

import (
	"bytes"
	"io"
	"testing"
)

func TestNewSizeAndBasicProps(t *testing.T) {
	b := NewSize(16)
	if b == nil {
		t.Fatal("NewSize returned nil")
	}
	if b.Len() != 0 {
		t.Fatalf("expected Len=0, got %d", b.Len())
	}
	if b.Cap() < 16 {
		t.Fatalf("expected Cap>=16, got %d", b.Cap())
	}
	if !b.IsEmpty() {
		t.Fatalf("expected IsEmpty=true")
	}
}

func TestFromBytes(t *testing.T) {
	src := []byte("hello world")
	b := FromBytes(src)
	if b.Len() != len(src) {
		t.Fatalf("expected Len=%d, got %d", len(src), b.Len())
	}
	if !bytes.Equal(b.Bytes(), src) {
		t.Fatalf("Bytes mismatch: got=%q, want=%q", string(b.Bytes()), string(src))
	}
	// Modify through Buffer, should reflect in src.
	b.Bytes()[0] = 'H'
	if src[0] != 'H' {
		t.Fatalf("expected src[0] = 'H', got %q", src[0])
	}
}

func TestWriteAndRead(t *testing.T) {
	b := NewSize(4)
	data := []byte("abcd")
	n, err := b.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Write n=%d, want=%d", n, len(data))
	}
	if b.Len() != len(data) {
		t.Fatalf("Len=%d, want=%d", b.Len(), len(data))
	}

	buf := make([]byte, 10)
	n, err = b.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Read n=%d, want=%d", n, len(data))
	}
	if string(buf[:n]) != "abcd" {
		t.Fatalf("Read content=%q, want %q", string(buf[:n]), "abcd")
	}
	// Second read should be EOF.
	n, err = b.Read(buf)
	if err != io.EOF {
		t.Fatalf("expected EOF, got err=%v, n=%d", err, n)
	}
}

func TestWriteGrow(t *testing.T) {
	b := NewSize(2)
	data := []byte("abcdefg")
	n, err := b.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Write n=%d, want=%d", n, len(data))
	}
	if b.Len() != len(data) {
		t.Fatalf("Len=%d, want=%d", b.Len(), len(data))
	}
	if !bytes.Equal(b.Bytes(), data) {
		t.Fatalf("Bytes mismatch: got=%q, want=%q", string(b.Bytes()), string(data))
	}
	if b.Cap() < len(data) {
		t.Fatalf("Cap=%d, expected >=%d", b.Cap(), len(data))
	}
}

func TestWriteByteAndReadByte(t *testing.T) {
	b := NewSize(0)
	for i := 0; i < 5; i++ {
		if err := b.WriteByte(byte('0' + i)); err != nil {
			t.Fatalf("WriteByte error at %d: %v", i, err)
		}
	}
	if b.Len() != 5 {
		t.Fatalf("Len=%d, want=5", b.Len())
	}

	for i := 0; i < 5; i++ {
		c, err := b.ReadByte()
		if err != nil {
			t.Fatalf("ReadByte error at %d: %v", i, err)
		}
		if c != byte('0'+i) {
			t.Fatalf("ReadByte got=%q, want=%q", c, byte('0'+i))
		}
	}

	_, err := b.ReadByte()
	if err != io.EOF {
		t.Fatalf("expected EOF from ReadByte, got %v", err)
	}
}

func TestExtend(t *testing.T) {
	b := NewSize(2)
	if _, err := b.Write([]byte("ab")); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	ext := b.Extend(3)
	if len(ext) != 3 {
		t.Fatalf("Extend len=%d, want=3", len(ext))
	}
	ext[0] = 'c'
	ext[1] = 'd'
	ext[2] = 'e'

	if b.Len() != 5 {
		t.Fatalf("Len=%d, want=5", b.Len())
	}
	if string(b.Bytes()) != "abcde" {
		t.Fatalf("Bytes=%q, want %q", string(b.Bytes()), "abcde")
	}
}

func TestTo(t *testing.T) {
	b := FromBytes([]byte("hello"))
	head2 := b.To(2)
	if string(head2) != "he" {
		t.Fatalf("To(2)=%q, want %q", string(head2), "he")
	}
	head10 := b.To(10)
	if string(head10) != "hello" {
		t.Fatalf("To(10)=%q, want %q", string(head10), "hello")
	}
}

func TestReadBytes(t *testing.T) {
	b := FromBytes([]byte("abcdef"))
	part, err := b.ReadBytes(3)
	if err != nil {
		t.Fatalf("ReadBytes error: %v", err)
	}
	if string(part) != "abc" {
		t.Fatalf("ReadBytes got=%q, want %q", string(part), "abc")
	}
	if b.Len() != 3 {
		t.Fatalf("remaining Len=%d, want=3", b.Len())
	}
	part, err = b.ReadBytes(3)
	if err != nil {
		t.Fatalf("ReadBytes second error: %v", err)
	}
	if string(part) != "def" {
		t.Fatalf("ReadBytes second got=%q, want %q", string(part), "def")
	}
	if !b.IsEmpty() {
		t.Fatalf("buffer should be empty after reading all")
	}

	b2 := FromBytes([]byte("xy"))
	_, err = b2.ReadBytes(3)
	if err != io.EOF {
		t.Fatalf("expected EOF when ReadBytes(3) from len=2, got %v", err)
	}
}

func TestReset(t *testing.T) {
	b := NewSize(8)
	if _, err := b.Write([]byte("1234")); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	b.Reset()
	if !b.IsEmpty() {
		t.Fatalf("expected empty after Reset")
	}
	if b.Len() != 0 {
		t.Fatalf("Len=%d, want=0", b.Len())
	}

	if _, err := b.Write([]byte("abcd")); err != nil {
		t.Fatalf("Write after Reset error: %v", err)
	}
	if string(b.Bytes()) != "abcd" {
		t.Fatalf("Bytes after Reset/Write=%q, want %q", string(b.Bytes()), "abcd")
	}
}

func TestReleaseNoPanic(t *testing.T) {
	b1 := NewSize(10)        // pooled buffer
	b2 := FromBytes([]byte("x")) // non-pooled

	b1.Release()
	b2.Release()
}
