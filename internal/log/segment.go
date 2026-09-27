package log

import(
	"fmt"
	"os"
	"path"

	api "github.com/alderon07/log/api/v1"
	"google.golang.org/protobuf/proto"
)

// The segment wraps the index and store types to coordinate operations across the two. For example, when the log appends a record to the active segment, the segment needs to write the data to its store and add a new entry in the index. Similarly for reads, the segment needs to look up the entry from the index and then fetch the data from the store.
type segment struct {
	store *store
	index *index
	baseOffset, nextOffset uint64
	config Config
}
