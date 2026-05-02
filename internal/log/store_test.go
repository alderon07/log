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

	testAppend(t, s)
	testRead(t, s)
	testReadAt(t, s)
	
	s, err := newStore(f)
	require.NoError(t, err)
	testRead(t, s)
}

func testAppend(t *testing.T, s store){
	t.Helper()
	for i := uint64(1); i < 4; i++ {
		n, pos, err := s.Append(write)
		require.NoError(t, err)
		require.Equal(t, pos + n, width * i)
	}
}