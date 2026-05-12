package log

import (
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
	require.NoError(t, err)
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
		_, pos, err := s.Append(write)
		require.NoError(t, err)
		// End of this record on disk: pos + 8-byte prefix + payload (n).
		require.Equal(t, pos + width, width * i)
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

	for i, offset := uint(1), int64(0); i < 4; i++ {
		b := make([]byte, metadataWidth)

		n, err := s.ReadAt(offset, b)
		require.NoError(t, err)
		require.Equal(t, metadataWidth, n)
		
		offset += int64(n)

		
		size := encoding.Uint64(b)	// get payload width from the metadata
		b = make([]byte, size)
		
		n, err = s.ReadAt(offset, b)
		require.NoError(t, err)
		require.Equal(t, int(size), n , "Read payload size should match the size extracted from metadata")
		
		offset += int64(n)
	}
}


func testStoreClose(t *testing.T){
	f, err := os.CreateTemp("", "store_close_test")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	store, err := newStore(f)
	require.NoError(t, err)
	
	_, _, err = store.Append(write)
	require.NoError(t, err)

	f, beforeSize, err := openFile(f.Name())
	require.NoError(t, err)
	
	err = store.Close()				// close store
	require.NoError(t, err)

	_, afterSize, err := openFile(f.Name())
	require.NoError(t, err)

	require.True(t, afterSize > beforeSize, "The file, store writes to, should be bigger after data is appended to it")
}

// openFile opens or creates the file at name for read/write append
// (O_RDWR|O_CREATE|O_APPEND) and returns its current byte length from Stat.
//
// Inputs:
//   - name: filesystem path to open; created with mode 0644 (owner read/write, group/others read) if it does not exist.
//
// Outputs:
//   - file: the opened *os.File; nil if OpenFile or Stat fails.
//   - size: fileInfo.Size() after open; 0 on error.
//   - err: non-nil if OpenFile or Stat fails.
func openFile(name string) (file *os.File, size int64, err error) {
	f, err := os.OpenFile(
		name,
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0644,
	)

	if err != nil {
		return nil, 0, err
	}

	fileInfo, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}

	return f, fileInfo.Size(), nil
}
	




	