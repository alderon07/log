package log

import (
	"io"
	"os"
	"testing"

	api "github.com/alderon07/log/api/v1"
	"github.com/stretchr/testify/require"
)

func TestSegment(t *testing.T) {
	dir, _ := os.MkdirTemp("", "segment-test")
	defer os.RemoveAll(dir)

	want := &api.Record{Value: []byte("Hello, World!")}

	conf := Config{}
	conf.Segment.MaxStoreBytes = 1024
	conf.Segment.MaxIndexBytes = entWidth * 3

	seg, err := newSegment(dir, 16, conf)
	require.NoError(t, err)
	require.Equal(t, uint64(16), seg.nextOffset, seg.nextOffset)
	require.False(t, seg.IsMaxed())

	for i := uint64(0); i < 3; i++ {
		off, err := seg.Append(want)
		require.NoError(t, err)
		require.Equal(t, 16+i, off)

		got, err := seg.Read(off)
		require.NoError(t, err)
		require.Equal(t, want.Value, got.Value)
	}

	// maxed index
	_, err = seg.Append(want)
	require.Equal(t, io.EOF, err)

	require.True(t, seg.IsMaxed())

	conf.Segment.MaxStoreBytes = uint64(len(want.Value) * 3)
	conf.Segment.MaxIndexBytes = 1024

	// Each stored record also has an 8-byte length prefix, so three records use
	// more than len(want.Value)*3 bytes and the store remains maxed after reopening.
	seg, err = newSegment(dir, 16, conf)
	require.NoError(t, err)

	// maxed store
	require.True(t, seg.IsMaxed())

	err = seg.Remove()
	require.NoError(t, err)
	seg, err = newSegment(dir, 16, conf)
	require.NoError(t, err)
	require.False(t, seg.IsMaxed())
}
