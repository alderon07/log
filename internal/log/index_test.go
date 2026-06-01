package log

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIndexClose(t *testing.T) {
	f, err := os.CreateTemp("", "index_close_test")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	cfg := Config{Segment: Segment{MaxIndexBytes: 1024}}
	idx, err := newIndex(f, cfg)
	require.NoError(t, err)

	err = idx.Close()
	require.NoError(t, err)

	_, afterSize, err := openFile(f.Name())
	require.NoError(t, err)
	require.Equal(t, int64(0), afterSize)
	require.Less(t, afterSize, int64(cfg.Segment.MaxIndexBytes))
}

func TestIndexCloseTwice(t *testing.T) {
	f, err := os.CreateTemp("", "index_close_twice_test")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	idx, err := newIndex(f, Config{Segment: Segment{MaxIndexBytes: 1024}})
	require.NoError(t, err)

	require.NoError(t, idx.Close())
	require.ErrorIs(t, idx.Close(), os.ErrClosed)
}
