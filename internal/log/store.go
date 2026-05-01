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
	mu sync.Mutex
	buf *bufio.Writer
	size uint64
}

func newStore(f *os.File) (*store, error){
	// 
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}

	// get file current size in case we're recreating store from a file with existing data on service restarts
	size := uint64(fi.Size())
	
	return &store{
		File: f,
		size: size,
		buf : bufio.NewWriter(f),
	}, nil
}

// Record layout on disk:
// [8-byte metadata length prefix][payload bytes]
//
// Write path:
// 1) binary.Write(..., uint64(len(payload))) writes fixed-width metadata bytes.
// 2) buf.Write(payload) writes raw payload bytes.

func(s *store) Append(payload []byte) (uint64, uint64, error){
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

// Read path:
// 1) Flush buffered writes so reads see latest appended records.
// 2) Read metadata at pos to know payload width.
// 3) Read payload at pos + metadataWidth.
//
// append data then return number of bytes and the position of said data
func(s *store) Read(pos uint64) ([]byte, error){
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

// reads metadata and record starting at pos given
func(s *store) ReadAt(pos uint64, bytes []byte) (int, error){
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.buf.Flush(); err != nil {
		return 0, err
	}

	return s.File.ReadAt(bytes, int64(pos))
}

func(s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()


	// flushing makes sure we write data from memory to disk before closing file
	if err := s.buf.Flush(); err != nil {
		return err
	}

	return s.File.Close()
}

