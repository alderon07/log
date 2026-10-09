package log

import (
	"os"
	"testing"

	api "github.com/alderon07/log/api/v1"
)

func TestSegment(t *testing.T) {
	dir, _ := os.MkdirTemp("", "segment-test")
	defer os.RemoveAll(dir)

	want := &api.Record{Value: []byte("Hello, World!")}

}
