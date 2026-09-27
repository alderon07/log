package log

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIndexClose(t *testing.T) {
	f, err := os.CreateTemp("", "index_close_test")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	cfg := Config{SegmentConfig{MaxIndexBytes: 1024}}
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

	idx, err := newIndex(f, Config{SegmentConfig{MaxIndexBytes: 1024}})
	require.NoError(t, err)

	require.NoError(t, idx.Close())
	require.ErrorIs(t, idx.Close(), os.ErrClosed)
}

func TestIndex(t *testing.T){
	f, err := os.CreateTemp(os.TempDir(), "index_test")
	require.NoError(t, err)

	defer os.Remove(f.Name())

	c := Config{}
	c.Segment.MaxIndexBytes = 1024
	
	idx, err := newIndex(f, c)
	require.NoError(t, err)

	_, _, err = idx.Read(-1)
	require.Error(t, err)

	require.Equal(t, f.Name(), idx.Name())

	entries := []struct{
		RecordID uint32
		Pos uint64
	}{
		{RecordID: 0, Pos: 0},
		{RecordID: 1, Pos: 10},
	}

	for _, entry := range entries{
		err = idx.Write(entry.RecordID, entry.Pos)
		require.NoError(t, err)

		_, pos, err := idx.Read(int64(entry.RecordID))
		require.NoError(t, err)

		require.Equal(t, entry.Pos, pos)
	}

	// index and scanner should error when reading past existing entries
	_, _, err = idx.Read(int64(len(entries)))
	require.NoError(t, io.EOF, err)
	_ = idx.Close()

	// index should build its state from the existing file
	f, _ = os.OpenFile(f.Name(), os.O_RDWR, 0600)
	idx, err = newIndex(f, c)
	require.NoError(t, err)

	off, pos, err := idx.Read(-1)
	require.NoError(t, err)
	require.Equal(t, uint32(1), off)
	require.Equal(t, entries[1].Pos, pos)
}