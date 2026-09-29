package log

import(
	"fmt"
	"os"
	"path"

	api "github.com/alderon07/log/api/v1"
	"google.golang.org/protobuf/proto"
)

// The segment wraps the index and store types to coordinate operations across the two. 
// For example, when the log appends a record to the active segment, the segment needs to write the data to its store and add a new entry in the index. Similarly for reads, the segment needs to look up the entry from the index and then fetch the data from the store.
type segment struct {
	store *store
	index *index
	baseOffset, nextOffset uint64
	config Config
}

func newSegment(dir string, baseOffset uint64, c Config) (*segment, error){
	seg := &segment{
		baseOffset: baseOffset,
		config: c,
	}

	var err error
	storeFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".store")),
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0644,
	)

	if err != nil {
		return nil, err
	}

	if seg.store, err = newStore(storeFile); err != nil {
		return nil, err
	}

	indexFile, err := os.OpenFile(
		path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".index")),
		os.O_RDWR|os.O_CREATE,
		// 0644 is an octal permission mode for a newly created file:
		// First digit 0: no special permission bits.
		// Owner digit 6 = read (4) + write (2).
		// Group digit 4 = read-only.
		// Others digit 4 = read-only.
		0644,
	)

	if err != nil{
		return nil, err
	}

	if seg.index, err = newIndex(indexFile, c); err != nil {
		return nil, err
	}

	if off, _, err := seg.index.Read(-1); err != nil {
		seg.nextOffset = baseOffset
	}else {
		seg.nextOffset = baseOffset + uint64(off) + 1
	}

	return seg, nil
}