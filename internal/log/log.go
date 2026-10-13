package log

import (
	"fmt"
	"os"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	api "github.com/alderon07/log/api/v1"
)

type Log struct {
	mu            sync.RWMutex
	Dir           string
	Config        Config
	activeSegment *segment
	segments      []*segment
}

// NewLog opens or initializes a log in dir using conf.
//
// Inputs:
//   - dir: directory containing the log's segment files.
//   - conf: segment size limits and the initial record offset.
//
// Outputs:
//   - *Log: the log instance.
//   - error: non-nil if the log directory cannot be read or initialized.
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

// setup loads the log's segments from its directory.
//
// Outputs:
//   - error: non-nil if the directory cannot be read or a segment cannot be created.
func (l *Log) setup() error {
	files, err := os.ReadDir(l.Dir)
	if err != nil {
		return err
	}

	baseOffsetSet := make(map[uint64]struct{})
	for _, file := range files {
		ext := path.Ext(file.Name())
		if ext != ".index" && ext != ".store" {
			continue
		}

		off, err := strconv.ParseUint(strings.TrimSuffix(file.Name(), ext), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid segment filename %q: %w", file.Name(), err)
		}
		baseOffsetSet[off] = struct{}{}
	}

	baseOffsets := make([]uint64, 0, len(baseOffsetSet))
	for off := range baseOffsetSet {
		baseOffsets = append(baseOffsets, off)
	}
	sort.Slice(baseOffsets, func(i, j int) bool {
		return baseOffsets[i] < baseOffsets[j]
	})

	for _, off := range baseOffsets {
		if err = l.newSegment(off); err != nil {
			return err
		}
	}

	if len(l.segments) == 0 {
		if err = l.newSegment(l.Config.Segment.InitialOffset); err != nil {
			return err
		}
	}
	return nil
}

func (l *Log) newSegment(offset uint64) error {
	seg, err := newSegment(l.Dir, offset, l.Config)
	if err != nil {
		return err
	}

	l.segments = append(l.segments, seg)
	l.activeSegment = seg

	return nil
}

// Append adds record to the active segment and returns its logical offset in
// the log. If the active segment reaches its configured size limit, Append
// creates a new segment starting at the next offset.
//
// Inputs:
//   - record: record data to append.
//
// Outputs:
//   - uint64: the record's logical offset in the log, not a byte position in a file.
//   - error: non-nil if appending the record or creating a new segment fails.
func (l *Log) Append(record *api.Record) (offset uint64, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	off, err := l.activeSegment.Append(record)
	if err != nil {
		return 0, err
	}

	if l.activeSegment.IsMaxed() {
		err = l.newSegment(off + 1)
	}

	return off, err
}

// Read returns the record at offset, its zero-based logical position in the
// log. Offset is a record number, not a byte position in a file.
//
// Inputs:
//   - offset: logical record offset to read.
//
// Outputs:
//   - *api.Record: the record at offset, or nil if no segment contains it.
//   - error: non-nil if offset is out of range or the record cannot be read.
func (l *Log) Read(offset uint64) (record *api.Record, err error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// find the correct segment
	var s *segment
	for _, segment := range l.segments {
		if offset >= segment.baseOffset && offset < segment.nextOffset {
			s = segment
			break
		}
	}

	if s == nil {
		return nil, fmt.Errorf("offset out of range: %d", offset)
	}

	return s.Read(offset)
}

// Close closes every segment in the log.
//
// Outputs:
//   - error: non-nil if closing any segment fails.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, segment := range l.segments {
		if err := segment.Close(); err != nil {
			return err
		}
	}

	return nil
}

// Remove closes the log and removes its directory and contents.
//
// Outputs:
//   - error: non-nil if closing the log or removing its directory fails.
func (l *Log) Remove() error {
	if err := l.Close(); err != nil {
		return err
	}
	return os.RemoveAll(l.Dir)
}

// Reset removes the log's existing files and initializes it again.
//
// Outputs:
//   - error: non-nil if removing or reinitializing the log fails.
func (l *Log) Reset() error {
	if err := l.Remove(); err != nil {
		return err
	}
	return l.setup()
}

func (l * Log) LowestOffset() (offset uint64, err error){
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.segments[0].baseOffset, nil
	
}

func (l *Log) HighestOffset() (offset uint64, err error){
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	lastIndex := len(l.segments) - 1
	off := l.segments[lastIndex].nextOffset

	if off == 0 {
		return 0, nil
	}

	return off - 1, nil
}

func (l *Log) Truncate(lowest uint64) error {
		l.mu.Lock()
		defer l.mu.Unlock()

		var s []*segment

		for _, segment := range l.segments {
			if segment.nextOffset <= lowest + 1 {
				if err := segment.Remove(); err != nil {
					return err
				}
				continue
			}
			s = append(s, segment)
		}

		l.segments = s
		return nil
}



func (l *Log) Reader() io.Reader {
	l.mu.RLock()
	defer l.mu.RUnlock()

	readers := make([]io.Reader, len(l.segments))
	for i, segment := range l.segments {
		readers[i] = &originReader{
			segment.store,
			0,
		}
	}

	return io.MultiReader(readers...)
}

type originReader struct {
	*store
	off int64
}

func (o *originReader) Read(p []byte) (int, error){
	n, err := o.ReadAt(o.off, p)
	o.off += int64(n)
	return n, err
}
