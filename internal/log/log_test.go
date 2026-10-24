package log

import (
	"io"
	"os"
	"testing"

	api "github.com/alderon07/log/api/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestLog(t *testing.T) {
	scenarios := map[string]func(t *testing.T, log *Log){
		"append and read a record succeeded": testAppendRead,
		"offset out of range error":          testOutOfRangeErr,
		"init with existing segments":        testInitExisting,
		"reader":                             testReader,
		"truncate":                           testTruncate,
	}

	for scenario, fn := range scenarios {
		t.Run(scenario, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "store-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			c := Config{}
			c.Segment.MaxStoreBytes = 32
			log, err := NewLog(dir, c)
			require.NoError(t, err)

			fn(t, log)
		})
	}
}

func testAppendRead(t *testing.T, log *Log) {
	r := &api.Record{
		Value: []byte("BAM!!"),
	}

	off, err := log.Append(r)
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	record, err := log.Read(off)
	require.NoError(t, err)
	require.Equal(t, record.Value, r.Value)
}

// tests that the log returns an error when we try to read an offset that’s outside of the range of offsets the log has stored
func testOutOfRangeErr(t *testing.T, log *Log) {
	read, err := log.Read(1)
	require.Nil(t, read)
	apiErr := err.(api.ErrOffsetOutOfRange)
	require.Equal(t, uint64(1), apiErr.Offset)
}

// When we create a log, the log should bootstrap itself from the data stored by prior log instances
func testInitExisting(t *testing.T, log *Log) {
	append := &api.Record{
		Value: []byte("BAM!!"),
	}

	for range 3 {
		_, err := log.Append(append)
		require.NoError(t, err)
	}

	require.NoError(t, log.Close())

	offset, err := log.LowestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(0), offset)

	offset, err = log.HighestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(2), offset)

	// check that the lowest and highest offsets are correct after reopening the log
	nLog, err := NewLog(log.Dir, log.Config)
	require.NoError(t, err)

	offset, err = nLog.LowestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(0), offset)

	offset, err = nLog.HighestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(2), offset)

	offset, err = nLog.Append(append)
	require.NoError(t, err)
	require.Equal(t, uint64(3), offset)
}

// tests that we can read the full, raw log as it’s stored on disk so that we can snapshot and restore the logs in ​Finite-State Machine​.
func testReader(t *testing.T, log *Log) {
	append := &api.Record{
		Value: []byte("BAM!!"),
	}

	_, err := log.Append(append)
	require.NoError(t, err)

	reader := log.Reader()
	b, err := io.ReadAll(reader)
	require.NoError(t, err)

	read := &api.Record{}
	err = proto.Unmarshal(b[metadataWidth:], read)
	require.NoError(t, err)
	require.Equal(t, append.Value, read.Value)
}

//  tests that we can truncate the log and remove old segments that we don’t need any more.
func testTruncate(t *testing.T, log *Log) {
	append := &api.Record{
		Value: []byte("BAM!!"),
	}

	for range 3 {
		_, err := log.Append(append)
		require.NoError(t, err)
	}

	err := log.Truncate(1)
	require.NoError(t, err)

	_, err = log.Read(0)
	require.Error(t, err)
}
