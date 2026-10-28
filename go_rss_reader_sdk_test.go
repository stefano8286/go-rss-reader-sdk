package gorssreadersdk

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempFile creates a file containing exactly the bytes in content and
// returns its path. The test process is responsible for cleanup via t.TempDir.
func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(name, content, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return name
}

func TestNewReaderRejectsNonPositiveSize(t *testing.T) {
	for _, size := range []int{-1, 0} {
		if _, err := NewReader("/dev/null", size); err == nil {
			t.Fatalf("expected error for size %d, got nil", size)
		}
	}
}

func TestNewReaderRejectsMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := NewReader(missing, 4); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestSizeReturnsConfiguredValue(t *testing.T) {
	r, err := NewReader(writeTempFile(t, []byte("abc")), 7)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	if got, want := r.Size(), 7; got != want {
		t.Fatalf("Size: got %d, want %d", got, want)
	}
}

func TestFileSizeMatchesWrittenContent(t *testing.T) {
	content := []byte("0123456789")
	r, err := NewReader(writeTempFile(t, content), 4)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	if got, want := r.FileSize(), int64(len(content)); got != want {
		t.Fatalf("FileSize: got %d, want %d", got, want)
	}
}

func TestReadFullChunk(t *testing.T) {
	r, err := NewReader(writeTempFile(t, []byte("ABCDEFGH")), 4)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	c, err := r.ReadChunk(0)
	if err != nil {
		t.Fatalf("ReadChunk: %v", err)
	}
	if c.EOF {
		t.Fatal("unexpected EOF on a full chunk")
	}
	if got, want := c.Offset, int64(0); got != want {
		t.Fatalf("Offset: got %d, want %d", got, want)
	}
	if got, want := c.Length(), 4; got != want {
		t.Fatalf("Length: got %d, want %d", got, want)
	}
	if got, want := string(c.Data), "ABCD"; got != want {
		t.Fatalf("Data: got %q, want %q", got, want)
	}
}

func TestReadMultipleChunksAcrossFile(t *testing.T) {
	r, err := NewReader(writeTempFile(t, []byte("ABCDEFGH")), 4)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	first, err := r.ReadChunk(0)
	if err != nil {
		t.Fatalf("ReadChunk(0): %v", err)
	}
	second, err := r.ReadChunk(4)
	if err != nil {
		t.Fatalf("ReadChunk(4): %v", err)
	}

	if string(first.Data) != "ABCD" {
		t.Fatalf("first.Data: got %q, want %q", first.Data, "ABCD")
	}
	if string(second.Data) != "EFGH" {
		t.Fatalf("second.Data: got %q, want %q", second.Data, "EFGH")
	}

	// Ownership: retaining first.Data must not be affected by reading second.
	if string(first.Data) != "ABCD" {
		t.Fatalf("first.Data mutated after second read: %q", first.Data)
	}
}

func TestReadShortFinalChunk(t *testing.T) {
	r, err := NewReader(writeTempFile(t, []byte("ABCDE")), 4)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	c, err := r.ReadChunk(4)
	if err != nil {
		t.Fatalf("ReadChunk: %v", err)
	}
	if c.EOF {
		t.Fatal("unexpected EOF on partial chunk")
	}
	if got, want := c.Length(), 1; got != want {
		t.Fatalf("Length: got %d, want %d", got, want)
	}
	if got, want := string(c.Data), "E"; got != want {
		t.Fatalf("Data: got %q, want %q", got, want)
	}
}

func TestReadAtFileSizeReturnsEOFChunk(t *testing.T) {
	content := []byte("ABC")
	r, err := NewReader(writeTempFile(t, content), 4)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	c, err := r.ReadChunk(int64(len(content)))
	if err != nil {
		t.Fatalf("ReadChunk: %v", err)
	}
	if !c.EOF {
		t.Fatal("expected EOF chunk at file end, got non-EOF")
	}
	if c.Length() != 0 {
		t.Fatalf("EOF chunk Length: got %d, want 0", c.Length())
	}
	if c.Data != nil {
		t.Fatalf("EOF chunk Data: got %v, want nil", c.Data)
	}
}

func TestReadPastFileSizeReturnsEOFChunk(t *testing.T) {
	r, err := NewReader(writeTempFile(t, []byte("ABC")), 2)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	c, err := r.ReadChunk(100)
	if err != nil {
		t.Fatalf("ReadChunk: %v", err)
	}
	if !c.EOF {
		t.Fatal("expected EOF chunk past file end, got non-EOF")
	}
}

func TestReadAtArbitraryOffset(t *testing.T) {
	content := []byte("0123456789ABCDEF")
	r, err := NewReader(writeTempFile(t, content), 4)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	c, err := r.ReadChunk(6)
	if err != nil {
		t.Fatalf("ReadChunk(6): %v", err)
	}
	if got, want := c.Offset, int64(6); got != want {
		t.Fatalf("Offset: got %d, want %d", got, want)
	}
	if got, want := string(c.Data), "6789"; got != want {
		t.Fatalf("Data: got %q, want %q", got, want)
	}
}

func TestNegativeOffsetReturnsError(t *testing.T) {
	r, err := NewReader(writeTempFile(t, []byte("ABC")), 2)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	if _, err := r.ReadChunk(-1); err == nil {
		t.Fatal("expected error for negative offset, got nil")
	}
}

func TestCloseIsIdempotentError(t *testing.T) {
	r, err := NewReader(writeTempFile(t, []byte("ABC")), 2)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close must surface an error rather than silently succeeding,
	// matching os.File.Close semantics.
	if err := r.Close(); err == nil {
		t.Fatal("expected error on second Close, got nil")
	}
}

func TestChunksDoNotShareBackingArray(t *testing.T) {
	r, err := NewReader(writeTempFile(t, []byte("ABCDEFGH")), 4)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	first, _ := r.ReadChunk(0)
	second, _ := r.ReadChunk(4)

	if first.EOF || second.EOF {
		t.Fatal("unexpected EOF")
	}
	// Mutating second.Data must not corrupt first.Data — each chunk owns an
	// independent buffer.
	for i := range second.Data {
		second.Data[i] = 'X'
	}
	if string(first.Data) != "ABCD" {
		t.Fatalf("first.Data corrupted by second.Data mutation: %q", first.Data)
	}
}
