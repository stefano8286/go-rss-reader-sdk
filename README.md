# go-rss-reader-sdk

Reads fixed-size chunks from files at arbitrary offsets with bounded memory and zero-copy semantics.

```go
package main

import (
	"fmt"

	gorssreadersdk "go-rss-reader-sdk"
)

func main() {
	r, err := gorssreadersdk.NewReader("/var/log/app.log", 4096)
	if err != nil {
		// handle missing file or invalid size
		return
	}
	defer r.Close()

	// Read a 4 KiB chunk starting at byte offset 8192.
	chunk, err := r.ReadChunk(8192)
	if err != nil {
		// handle seek or read error
		return
	}
	if chunk.EOF {
		// offset is at or beyond the file size; no data here
		return
	}
	fmt.Printf("offset=%d len=%d\n", chunk.Offset, chunk.Length())
	_ = chunk.Data // safe to retain; will not be overwritten by a later ReadChunk
}
```

## Why this exists

Streaming readers like `bufio.Scanner` hide the offset from you and usually
read sequentially. When a caller needs to fetch block N of a file by index,
jump into the middle of an append-only log, or parallelise reads across a
fixed-grid layout, they end up wrapping `os.File` in their own seek-plus-read
plumbing, and that plumbing tends to re-implement the same edge cases each
time: the short last chunk, the EOF-at-exact-boundary case, the negative
offset. This library exists to handle those once.

The trade-off: every `ReadChunk` allocates a fresh `[]byte` rather than
reusing an internal buffer. The win is that callers can hold a chunk without it
being silently overwritten by the next read — there is no hidden aliasing and
no `io.Reader`-style "consume this before the next call" contract to reason
about. The cost is one allocation per chunk. For a chunk size in the kilobytes
that is negligible; if you are reading millions of tiny chunks per second you
are better served by a pooled buffer.

## The awkward edge

`ReadChunk` distinguishes two end-of-file situations, and conflating them is
the mistake callers make most often:

- Reading at an offset **≥ file size** returns a chunk with `EOF == true` and
  `Length() == 0`. This is the "you are done" signal. There is no error,
  because reaching the end of a file is normal.
- Reading at an offset **< file size** where fewer than `Size` bytes remain
  returns a short chunk (`EOF == false`, `Length()` between 1 and `Size-1`).
  This is real data, not EOF. Treating it as EOF will drop the last record of
  any file whose length is not an exact multiple of the chunk size.

If you need EOF on the short final chunk, read once more at
`offset + chunk.Length()`; that call will then return `EOF == true`.

`NewReader` snapshots the file size via `os.Stat` at construction time. If the
file grows or shrinks underneath you, the EOF boundary reflects the size at
open, not at read. Re-open the reader to refresh it.
