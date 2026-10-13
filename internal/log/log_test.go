package log

import (
	"os"
	"testing"

	api "github.com/alderon07/log/api/v1"
	"github.com/stretchr/testify/require"
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
		Value: []byte("BAMM!"),
	}

	off, err := log.Append(r)
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	record, err := log.Read(off)
	require.NoError(t, err)
	require.Equal(t, record.Value, r.Value)
}
