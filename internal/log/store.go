package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

// ecoding we persist record sizes and index entries in
var (
	encoding = binary.BigEndian
)

// number of bytes used to store record's length
const (
	metadataWidth = 8
)

type store struct {
	*os.File
	mu   sync.Mutex
	buf  *bufio.Writer
	size uint64
}

// newStore wraps an open file as an append-only record store with a buffered writer.
// It sets the logical store size from the file's current length so reopening an
// existing file preserves offsets across restarts.
//
// Inputs:
//   - f: open *os.File that will back the store (typically read/write).
//
// Outputs:
//   - *store: initialized store with bufio.Writer and size from Stat.
//   - error: non-nil if Stat fails or the file cannot be inspected.
func newStore(f *os.File) (*store, error) {
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}

	// get file current size in case we're recreating store from a file with existing data on service restarts
	size := uint64(fi.Size())

	return &store{
		File: f,
		size: size,
		buf:  bufio.NewWriter(f),
	}, nil
}

// Append writes one record: an 8-byte big-endian length prefix followed by payload.
// It holds the store lock, writes through the buffered writer, and advances the
// in-memory size. Data may remain buffered until Flush (e.g. from Read, ReadAt, Close).
//
// Record layout on disk:
//
//	[8-byte metadata length prefix][payload bytes]
//
// Inputs:
//   - payload: raw record bytes to append after the length prefix.
//
// Outputs:
//   - uint64: payload byte count written (same as len(payload) on success).
//   - uint64: byte offset (pos) where this record starts in the file before the append.
//   - error: non-nil if writing metadata or payload fails.
func (s *store) Append(payload []byte) (uint64, uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos := s.size

	// write the length of data as metadata into the buffer. We have to convert our int to bytes first.
	if err := binary.Write(s.buf, encoding, uint64(len(payload))); err != nil {
		return 0, 0, err
	}

	// buffered write to reduce system calls and improve performance
	recordWidth, err := s.buf.Write(payload)
	if err != nil {
		return 0, 0, err
	}

	// add the metadata number of bytes to the number of bytes of data we wrote to the buffer
	size := recordWidth + metadataWidth
	s.size += uint64(size)

	return uint64(recordWidth), pos, nil
}

// Read loads a full record starting at pos: it reads the fixed-width length prefix,
// allocates a buffer of that length, then reads the payload. It flushes first so
// buffered appends are visible, and uses the store lock for the whole operation.
//
// Inputs:
//   - pos: file byte offset of the record's start (length prefix), as returned by Append.
//
// Outputs:
//   - []byte: decoded payload only (not including the 8-byte prefix).
//   - error: non-nil on flush failure, short read, or I/O error while reading metadata or payload.
func (s *store) Read(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// ensures any recent Append data in memory is actually written to disk before reading
	if err := s.buf.Flush(); err != nil {
		return nil, err
	}

	size := make([]byte, metadataWidth)
	metadataPos := int64(pos)
	// ReadAt puts the read data into size here
	if _, err := s.File.ReadAt(size, metadataPos); err != nil {
		return nil, err
	}

	recordBuffer := make([]byte, encoding.Uint64(size))
	recordPos := int64(pos + metadataWidth)
	if _, err := s.File.ReadAt(recordBuffer, recordPos); err != nil {
		return nil, err
	}

	return recordBuffer, nil
}

// ReadAt reads up to len(bytes) bytes from the store's backing file at offset,
// mirroring os.File.ReadAt. It acquires the store lock, flushes any buffered
// writes so reads see the latest appended data, then delegates to the file.
//
// Inputs:
//   - offset: absolute byte position in the file to read from.
//   - bytes: destination buffer; at most len(bytes) bytes are read into it.
//
// Outputs:
//   - int: number of bytes read (0 <= n <= len(bytes)).
//   - error: flush or read failure; may be io.EOF when n < len(bytes) at end of file.
func (s *store) ReadAt(offset int64, bytes []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.buf.Flush(); err != nil {
		return 0, err
	}

	return s.File.ReadAt(bytes, int64(offset))
}

// Close flushes any buffered writes to disk, then closes the underlying file.
// It holds the store lock for the duration; after a successful close the store
// must not be used again.
//
// Inputs:
//   - none: the method has no parameters other than the store receiver.
//
// Outputs:
//   - error: non-nil if Flush fails or the underlying file Close fails.
func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// flushing makes sure we write data from memory to disk before closing file
	if err := s.buf.Flush(); err != nil {
		return err
	}

	return s.File.Close()
}
