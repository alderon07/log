package log

import (
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Log struct {
	mu            sync.Mutex
	Dir           string
	Config        Config
	activeSegment *segment
	segments      []*segment
}

func NewLog(dir string, conf Config) (*Log, error) {
	if conf.Segment.MaxStoreBytes == 0 {
		conf.Segment.MaxStoreBytes = 1024
	}

	if conf.Segment.MaxIndexBytes == 0 {
		conf.Segment.MaxIndexBytes = 1024
	}

	log := &Log{
		Dir:    dir,
		Config: conf,
	}

	return log, log.setup()
}

func (l *Log) setup() error {
	files, err := os.ReadDir(l.Dir)
	if err != nil {
		return err
	}

	var baseOffsets []uint64
	for _, file := range files {
		offStr := strings.TrimSuffix(
			file.Name(),
			path.Ext(file.Name()),
		)
		off, _ := strconv.ParseUint(offStr, 10, 0)
		baseOffsets = append(baseOffsets, off)

		sort.Slice(baseOffsets, func(i, j int) bool {
			return baseOffsets[i] < baseOffsets[j]
		})

	}
}
