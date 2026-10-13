# Log Storage Notes

## Log and Segment Basics

A **log** here is a list of records that grows as new records are added. Each record has an **offset**, which is its number in the list. The offset lets the program find a record later.

As the list grows, it is split into **segments**. Each segment holds a consecutive group of records. For example, one segment might hold records 0–99 and the next records 100–199. Splitting the log this way keeps each group manageable.

A segment is a small Go object in the running program that coordinates two files:

- The **store** file holds the actual record data.
- The **index** file maps each record's segment-local offset to the byte position where its data starts in that segment's store.

When reading a record, the segment looks up where the record is in the index, then reads its data from the store file. The segment connects these two jobs; it does not mean the record data is all kept in memory. Details about the segment, such as which record numbers it covers, are held in the running program. The index and record data are kept in files.

## Architecture

- `store.go` contains the actual log records in an append-only store file.
- `index.go` provides a lookup from a segment-local record offset to the record's byte position in that segment's store file.
- The current HTTP server uses `internal/server/log.go`, which stores records in memory. It is not connected to `store.go` or `index.go` yet.

## Store File

Each record in the store file is saved as its length followed by its data. The length tells the program how many bytes belong to that record:

```text
[8-byte payload length][payload bytes]
```

For example, if one saved record takes 25 bytes in the file, the next one starts at byte 25:

```text
position 0:  [length][record 0 payload]
position 25: [length][record 1 payload]
position 58: [length][record 2 payload]
```

`store.go` adds records to the end of the file, reads records back, and keeps track of how much file space has been used.

## Record ID, Offset, and Position

These terms describe different things. “Offset” can mean a record's number in the log, while “position” means a byte location inside a file:

- **Log offset**: a record's number in the whole log, such as `0`, `1`, or `2`. `Log.Read` uses this number to find a record. The first number can be configured.
- **Segment offset**: a record's number counted from the start of its segment. For example, if a segment starts at log offset 10, then log offset 12 is the third record in that segment, with segment offset 2.
- **Index entry number**: the number of a row in a segment's index, starting from 0. Each row describes one record. `index.Read` uses this number; `-1` is a special request for the last row. In the current code, the row number matches the segment offset.
- **Store-file position**: the byte number in the store file where a record's saved data begins. The index keeps this number so the program can find the data quickly.
- **Record data**: the actual information saved for a record.

To find a record, the segment turns the log offset into a number within that segment, then looks up the corresponding row in the index. That row tells it where the record starts in the store file:

```text
log offset 12
  -> segment baseOffset 10, so segment-local offset / index entry number 2
  -> index entry 2 returns store-file position 58
  -> read the record starting at byte 58 in that segment's store file
```

Records can be different sizes, so their numbers do not tell us where they start in a file. The index provides that byte position.

## Index

Each segment has its own index file. It contains one same-sized row for each record. Each row stores:

```text
[4-byte segment-local record offset][8-byte store-file position]
```

Each row takes 12 bytes:

```go
offsetWidth = 4
posWidth    = 8
entWidth    = offsetWidth + posWidth // 12
```

To find row number `n`, the program starts `n * 12` bytes from the beginning of the index file:

```text
n * entWidth
```

For example:

```text
Entry 0: bytes 0-11
Entry 1: bytes 12-23
Entry 2: bytes 24-35
```

To find a record, `segment.Read` subtracts the segment's starting log offset (`baseOffset`) from the requested log offset. That gives the row number to look up. The index returns the record's segment offset and its starting byte in the store file. The segment then reads the record data and turns it back into a record the program can use.

```text
log offset -> segment-local offset / index entry number
           -> stored store-file position -> record data
```

`index.Write` adds a row to the index. `index.Read(n)` reads row `n`; `index.Read(-1)` reads the last row. If the index is empty or the requested row is outside the part that contains saved rows, `Read` returns `io.EOF` (Go's standard “no more data here” error). The first value returned by `Read` is the segment offset saved in that row; it is not the row number requested.

## Segment Offsets

The log is split into segments, each covering a consecutive range of logical record offsets:

```text
Log offsets:   10   11   12   13   14   15
               |---- Segment A ----|---- Segment B ----|

Segment A:
  baseOffset = 10                    nextOffset = 13
  contains:   [10, 11, 12]           excludes 13

Segment B:
                                    baseOffset = 13
                                    nextOffset = 16
                                    contains:   [13, 14, 15]
```

`baseOffset` is the first log offset in a segment. `nextOffset` is the next log offset that would be assigned if a record were added to that segment. For example, if `baseOffset` is 10 and `nextOffset` is 13, the segment contains records 10, 11, and 12. In code, this range is written:

```text
baseOffset <= offset < nextOffset
```

When a segment fills, the next segment starts at the previous segment's `nextOffset`. `Log.Read(offset)` finds the segment that contains the requested record number, then asks that segment to read it.

## Growing and Truncating the Index

When a segment is opened, it needs to work out which log offset to use for the next record. `newIndex` remembers how many bytes the index file currently uses, then makes the file large enough for the configured maximum number of rows. The segment asks the index for its last row. If a row exists, it uses the saved segment offset to work out the next log offset. If the index is empty, it starts at `baseOffset`.

Before the index is memory-mapped, the file is grown to its maximum configured size:

```text
[real index entries][empty reserved space]
```

The extra space is reserved so the index can grow without resizing the file while it is in use. The file is made large enough before it is connected to memory.

This means the end of the file may be unused space rather than the last row. The program must track how much of the file contains actual rows; it cannot assume the last row is at the very end of the file.

During a graceful shutdown, `index.Close()` truncates the file back to its logical size:

```text
Before shutdown: [real entries][empty reserved space]
After shutdown:  [real entries]
```

This removes the unused space so the file contains only the rows that were written. When the program starts again, the segment can find its last row and work out which log offset comes next.

### Ungraceful Shutdowns

If the program stops unexpectedly before shrinking the file, the index may still look like this:

```text
[real index entries][empty reserved space]
```

The program cannot safely assume that the last 12 bytes contain a real row. A more complete system could check the index, remove unused space, rebuild the index from the record file, or restore it from another copy.

This project keeps things simple and does not yet repair the index after an unexpected stop.

## Buffered Writer

`store.go` uses a `bufio.Writer`, a small temporary area in memory that collects data before writing it to the file:

```text
Append -> Go memory buffer -> Flush -> file/OS cache
```

Collecting several small writes before sending them to the file can make writing more efficient.

The tradeoff is that data still waiting in this temporary area can be lost if the program stops before sending it to the file. `Flush()` sends the waiting data to the file.

## File Handle

A `*os.File` is not a memory buffer and does not contain another copy of the file. It is the program's handle for working with an open file.

Through this handle, the program can:

```go
Read
ReadAt
Write
Sync
Close
```

## `Flush` and `Sync`

They do different jobs:

```text
bufio buffer --Flush()--> OS cache --Sync()--> stable storage
```

- `Flush()` sends waiting data from the program's temporary buffer to the file.
- `Sync()` asks the operating system to save file changes to the storage device.
- `Close()` closes the program's connection to the file.

`Sync()` helps protect changes if the machine loses power, but can take longer because it may wait for the storage device.

## Memory-Mapped File

A **memory-mapped file** is a file that the program can work with as if its contents were a byte array in memory:

```text
index file <-> mapped memory
```

Instead of asking the operating system to read or write each small piece separately, the program can access the mapped bytes directly. The operating system brings the needed parts into memory and manages writing changes back to the file.

This is different from the temporary buffer used by `bufio.Writer`:

- `bufio.Writer`: temporary data waiting to be written to the file.
- `mmap.MMap`: a way to access the file's contents through memory.

The operating system may keep parts of the mapped file in memory for a while. Using a memory mapping does not by itself mean every change has been saved to the storage device.

### `mmap.Flush()` versus `file.Sync()`

When the program changes mapped bytes, those changes may first be waiting in memory:

```text
mapped memory --mmap.Flush()--> file/OS cache --file.Sync()--> stable storage
```

- `mmap.Flush()` asks that changes made through the memory mapping be written to the file.
- `file.Sync()` asks the operating system to save the file's contents and details to the storage device.

In `index.go`, memory mapping lets the program quickly jump to a row in the index. Because every row is 12 bytes, row `n` starts 12 times `n` bytes from the beginning.

## Adapter Pattern

An **adapter** is a small wrapper that makes an existing value usable where a different interface is expected. The existing value does not need to change; the adapter translates calls between the two shapes.

```text
what the caller expects -> adapter -> what we already have
```

A practical way to build one:

1. Identify the interface the caller needs.
2. Identify the methods or behavior the existing type provides.
3. Write a wrapper that implements the needed interface by calling the existing methods.
4. Store any extra state needed to translate behavior, such as a current read position.

### Example 1: Read a Store Sequentially

In this log, `store` supports `ReadAt(offset, buffer)`, which reads from a chosen byte position. `io.MultiReader` needs values that implement `io.Reader`, whose `Read` method reads from the current position and advances it. `originReader` adapts between them by keeping its own position:

```go
type originReader struct {
    store *store
    off   int64
}

func (o *originReader) Read(p []byte) (int, error) {
    n, err := o.store.ReadAt(o.off, p)
    o.off += int64(n)
    return n, err
}
```

`Log.Reader` creates one `originReader` per segment and combines them with `io.MultiReader`, so callers can read the whole log as one sequential byte stream. This capability will be useful when consensus support needs to create snapshots or restore a log.

The wrappers serve two purposes:

1. They implement `io.Reader`, which is the interface required by `io.MultiReader`. A store supports `ReadAt`, so `originReader.Read` adapts that operation into sequential reads.
2. Each wrapper starts with `off` set to zero. Every `Read` advances `off` by the number of bytes read, so reading begins at the start of each store and continues through its contents until EOF.

`io.MultiReader` exhausts one segment's store before moving on to the next, presenting all the segment stores as a single stream.

### Example 2: Adapt a Function to an HTTP Handler

`net/http` expects an `http.Handler`, an interface with a `ServeHTTP` method. The standard library's `http.HandlerFunc` adapts a regular function to that interface:

```go
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "Hello")
})

http.Handle("/", handler)
```

The adapter lets a function be used anywhere an `http.Handler` is expected.

### Example 3: Adapt an Existing Logger to `io.Writer`

Suppose an existing logger accepts strings, while another API accepts an `io.Writer`:

```go
type oldLogger interface {
    Log(string)
}

type loggerWriter struct {
    logger oldLogger
}

func (w loggerWriter) Write(p []byte) (int, error) {
    w.logger.Log(string(p))
    return len(p), nil
}
```

Now `loggerWriter` can be passed to code that expects an `io.Writer`, such as `fmt.Fprintln`. This simple adapter treats each `Write` call as one message.

## Simple Summary

```text
store.go:
    stores record data

index.go:
    maps segment-local record offsets to store-file positions

bufio.Writer:
    temporary Go memory for efficient sequential writes

mmap.MMap:
    memory view of file-backed index bytes

log offset:
    logical record number across the whole log

segment-local offset:
    logical record number relative to one segment; stored in the index

position:
    byte location in a segment's store file

index entry number:
    zero-based row number within one segment's index; -1 selects the last row

entWidth:
    12 bytes, the size of one index entry
```
