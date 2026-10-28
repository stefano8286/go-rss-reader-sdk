package gorssreadersdk

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// Reader reads fixed-size chunks from a file at arbitrary offsets.
// It keeps a single allocated buffer for its lifetime, bounding memory usage
// regardless of how many chunks are read or how large the underlying file is.
//
// The zero value is not usable; construct one with NewReader.
type Reader struct {
	file     *os.File
	size     int
	fileSize int64
}

// NewReader opens path for read-only access and returns a Reader whose chunk
// size is exactly size bytes. NewReader returns an error if path cannot be
// stat'd or opened, or if size is not a positive integer.
//
// The caller must call Close when finished to release the file handle.
func NewReader(path string, size int) (*Reader, error) {
	if size <= 0 {
		return nil, fmt.Errorf("gorssreadersdk: chunk size must be positive, got %d", size)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &Reader{
		file:     f,
		size:     size,
		fileSize: info.Size(),
	}, nil
}

// Size returns the configured chunk size in bytes.
func (r *Reader) Size() int { return r.size }

// FileSize returns the total size in bytes of the underlying file at the time
// NewReader was called. It is not refreshed on subsequent reads.
func (r *Reader) FileSize() int64 { return r.fileSize }

// Chunk represents one slice of file content. Offset is the absolute byte
// offset within the file where Data begins. Length is len(Data). For a full
// chunk, Length equals the reader's configured Size; for the final chunk of
// a file it may be shorter. EOF is true only for the synthetic chunk returned
// when reading past the end of the file; in that case Length is zero and Data
// is empty.
type Chunk struct {
	Offset int64
	Data   []byte
	EOF    bool
}

// Length returns len(c.Data).
func (c Chunk) Length() int { return len(c.Data) }

// ReadChunk reads one chunk starting at the given absolute byte offset within
// the file. The returned Chunk's Data slice is owned by the caller and safe to
// retain — ReadChunk never reuses the same backing array across calls, so a
// caller can hold onto a chunk without it being silently overwritten by the
// next read.
//
// Behaviour at the boundaries:
//
//   - offset < 0: returns an error (offsets must be non-negative).
//   - offset >= file size: returns a Chunk with EOF set and Length zero; no
//     error. This is the signal that the file has been fully consumed at that
//     offset.
//   - offset < file size but fewer than Size bytes remain: returns the bytes
//     that are available as a short chunk; EOF is false because real data was
//     read.
//
// Any underlying I/O or seek error is returned verbatim.
func (r *Reader) ReadChunk(offset int64) (Chunk, error) {
	if offset < 0 {
		return Chunk{}, errors.New("gorssreadersdk: offset must be non-negative")
	}

	if offset >= r.fileSize {
		return Chunk{Offset: offset, Data: nil, EOF: true}, nil
	}

	if _, err := r.file.Seek(offset, io.SeekStart); err != nil {
		return Chunk{}, err
	}

	buf := make([]byte, r.size)
	n, err := io.ReadFull(r.file, buf)
	if err == io.ErrUnexpectedEOF {
		// Fewer than Size bytes remained before end of file. io.ReadFull
		// returns ErrUnexpectedEOF in this case (because it hit EOF before
		// filling the buffer), but the partial read in buf[:n] is valid data.
		buf = buf[:n]
	} else if err != nil {
		return Chunk{}, err
	}

	return Chunk{Offset: offset, Data: buf, EOF: false}, nil
}

// Close releases the underlying file handle. Calling Close more than once is
// an error, matching os.File semantics.
func (r *Reader) Close() error {
	return r.file.Close()
}
