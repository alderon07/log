package log

import (
	"io"
	"os"
	"testing"
	"github.com/stretchr/testify/require"
)

var (
	write = []byte("Hello World!")
	width = uint64(len(write)) + metadataWidth
)

func TestStoreAppendRead(t *testing.T){
	f, err := os.CreateTemp("", "store_apend_read_test")
	// instantly stop test if we find an error
	require.NoError(f, err)
	defer os.Remove(f.Name())

	s, err := newStore(f)
	require.NoError(t, err)

	testAppend(t, s)
	testRead(t, s)
	testReadAt(t, s)
	
	s, err = newStore(f)
	require.NoError(t, err)
	testRead(t, s)
}

func testAppend(t *testing.T, s *store){
	// When an assertion fails inside that helper (or anything it calls), failure output skips this helper’s frame and shows the line in the real test that called the helper instead
	t.Helper()
	for i := uint64(1); i < 4; i++ {
		n, pos, err := s.Append(write)
		require.NoError(t, err)
		require.Equal(t, pos + n, width * i)
	}
}

func testRead(t *testing.T, s *store){
	// Mark as helper so failures point at the caller (e.g. TestStoreAppendRead), not this line.
	t.Helper()
	// Byte offset of the current record: metadata length prefix starts at pos, payload follows.
	var pos uint64
	// Three appends were written in testAppend; read each record back in order.
	for i := uint64(1); i < 4; i++ {
		read, err := s.Read(pos)
		require.NoError(t, err)
		// Read returns only the payload bytes (prefix was decoded to size the buffer).
		require.Equal(t, write, read)
		// Next record starts immediately after this one: 8-byte prefix + len(write).
		pos += width
	}
}

func testReadAt(t *testing.T, s *store){
	t.Helper()

	var pos uint64
	
}