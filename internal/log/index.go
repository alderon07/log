package log

import (
	"os"

	"github.com/edsrzf/mmap-go"
)

// Index entry layout (fixed-width rows in the index file).
//
// Each on-disk entry is entWidth bytes: a uint32 "offset" field plus a uint64
// "position" field (big-endian in the serialized form, matching the store).
//
// Offset vs position (naming used by this package’s index):
//
//   - Offset (offsetWidth bytes): logical record identifier — which record in the
//     append-only sequence (often the same idea as the record’s index or ordinal).
//     It is not a byte offset into the store file; it fits in fewer bytes because
//     it counts records, not bytes.
//
//   - Position (posWidth bytes): byte offset in the store file where that record
//     begins (the value Append returns as the record’s starting file position).
//     Large records mean positions can grow quickly, hence uint64.
//
//   - entWidth: total bytes per index row. To locate the i-th index entry’s bytes
//     inside the index file, use i * entWidth (a dense table of fixed-size entries).
//
// The *Width names are the serialized field sizes so layout stays easy to spot
// at the top of the file, mirroring metadataWidth in the store.
var (
	offsetWidth uint64 = 4
	posWidth    uint64 = 8
	entWidth    uint64 = offsetWidth + posWidth
)

// index keeps a memory-mapped view of the index file alongside the OS file handle.
//
// Memory mapping (mmap)
//
// Mmap asks the kernel to map a range of bytes from a file (or anonymous memory)
// directly into the process’s virtual address space. Reads and writes to that
// address range are translated by the OS into page-sized reads/writes against
// the underlying file (or swap), often with read-ahead and the buffer cache,
// instead of copying through a user-space buffer for every read/write syscall pair.
//
// What it is used for
//
//   - Fast random access to large read-mostly or read/write files (indexes, column
//     chunks) where touching many scattered offsets would otherwise mean lots of
//     pread/pwrite calls.
//   - Sharing read-only data between processes (code, shared libraries, mapped
//     configuration) with a single physical page cache backing.
//   - Latency-sensitive paths where avoiding extra copies matters, subject to
//     correct msync/fsync discipline when durability guarantees are required.
//
// Practical implementations (examples)
//
//   - Databases and logs: SQLite, LMDB, and many embedded engines map B-tree or
//     log index pages; this project can map an index file while a separate store
//     file holds record payloads.
//   - Search and analytics: memory-map inverted lists or columnar segments
//     (e.g. parts of the Lucene family of designs) for efficient scans.
//   - Language runtimes and tools: the Go runtime memory-maps executable segments;
//     debuggers and profilers map object files for symbol tables.
//   - IPC: POSIX shared memory (shm_open) is anonymous or file-backed mmap
//     between cooperating processes.
//
// In Go, libraries such as github.com/edsrzf/mmap-go wrap syscall.Mmap (or
// equivalent) with a byte slice–like API (MMap) and helpers for safe unmap.
type index struct {
	file *os.File
	mmap mmap.MMap
	size uint64
}

func newIndex(f *os.File, c Config) (*index, error){
	idx := &index{
		file: f,
	}
	
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err	
	}

	idx.size = uint64(fi.Size())
	if err = os.Truncate(f.Name(), int64(c.Segment.MaxIndexBytes)); err != nil {
		return nil, err
	}
	
	if idx.mmap, err = mmap.Map(idx.file, mmap.RDWR, 0); err != nil {
		return nil, err
	}

	return idx, nil
}

func (i *index) Close() error {
	// synchronizes the data in memory with data on disk
	if err := i.mmap.Flush(); err != nil {
		return err 
	}

	if err := i.file.Sync(); err != nil {
		return err
	}

	if err := i.file.Truncate(int64(i.size)); err != nil {
		return err
	}
	
	return i.file.Close()
}



type Config struct {
	segment Segment
}
