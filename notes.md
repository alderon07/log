# Log Storage Notes

## Architecture

- `store.go` contains the actual log records in an append-only store file.
- `index.go` is intended to provide a fast lookup from a logical record identifier to the record's byte location in the store file.
- The current HTTP server uses `internal/server/log.go`, which stores records in memory. It is not connected to `store.go` or `index.go` yet.

## Store File

Each record in the store file has this layout:

```text
[8-byte payload length][payload bytes]
```

For example:

```text
position 0:  [length][record 0 payload]
position 25: [length][record 1 payload]
position 58: [length][record 2 payload]
```

`store.go` appends records, reads complete records, reads arbitrary byte ranges, and tracks the current file size.

## Record ID, Offset, and Position

These terms describe different things:

- **Record ID**: the logical identifier of a record, such as `0`, `1`, or `2`.
- **Offset**: it's basically a record id. `record_id` would be a clearer name because `offset` often means a byte location.
- **Position**: the physical byte location where the record starts in the store file.
- **Payload**: the actual record data.

The index translates a logical identifier into a file position:

```text
record_id 2 -> position 58 -> read the record starting at byte 58
```

Because records can have different sizes, a record ID cannot be used as a byte position.

## Index

The index stores fixed-size entries containing:

```text
[4-byte record identifier][8-byte store position]
```

Each index entry is 12 bytes wide:

```go
offsetWidth = 4
posWidth    = 8
entWidth    = offsetWidth + posWidth // 12
```

The entry for index `n` starts at:

```text
n * entWidth
```

For example:

```text
Entry 0: bytes 0-11
Entry 1: bytes 12-23
Entry 2: bytes 24-35
```

The intended lookup flow is:

```text
record ID -> index entry -> store position -> read record
```

`index.Write` and `index.Read` are currently stubs, so this lookup is not implemented yet.

## Growing and Truncating the Index

When the service starts, it needs to know the next record ID. It can find that by reading the final 12-byte entry in the index file.

Before the index is memory-mapped, the file is grown to its maximum configured size:

```text
[real index entries][empty reserved space]
```

The extra space is reserved because a memory-mapped file cannot normally be resized while it is mapped. Growing the file must happen before mapping it.

The reserved space creates a problem: the final 12 bytes of the file are now empty space, not the last real index entry. The service can no longer find the final entry by simply reading the end of the file.

During a graceful shutdown, `index.Close()` truncates the file back to its logical size:

```text
Before shutdown: [real entries][empty reserved space]
After shutdown:  [real entries]
```

This removes the unused space and puts the last real entry at the end of the file again. The service can then restart and find the next record ID efficiently.

### Ungraceful Shutdowns

If the service crashes or loses power before truncation, the index may still look like this:

```text
[real index entries][empty reserved space]
```

The service cannot safely assume that the final 12 bytes contain a valid entry. A production system could validate the index, remove invalid trailing space, rebuild it from the store file, or replicate it from a healthy source.

This project keeps the implementation simple and does not handle recovery from an ungraceful shutdown.

## Buffered Writer

`store.go` uses a `bufio.Writer`:

```text
Append -> Go memory buffer -> Flush -> file/OS cache
```

The buffer is separate memory that temporarily holds writes. It allows multiple small writes to be combined, reducing system calls and improving performance.

The tradeoff is that data in the buffer can be lost if the process crashes before it is flushed.

## File Handle

A `*os.File` is not a memory buffer and does not contain a separate copy of the file. It is a handle representing an open operating-system file.

The handle lets the program perform operations such as:

```go
Read
ReadAt
Write
Sync
Close
```

## `Flush` and `Sync`

They operate at different layers:

```text
bufio buffer --Flush()--> OS cache --Sync()--> stable storage
```

- `Flush()` moves data from the Go `bufio.Writer` into the file and operating-system cache.
- `Sync()` asks the operating system to persist pending file changes to stable storage.
- `Close()` releases the file handle after pending work has been handled.

`Sync()` improves durability but can be slower because it may wait for the storage device.

## Memory-Mapped File

A memory-mapped view makes a region of a file accessible through memory addresses, like a byte array:

```text
index file <-> mapped memory
```

Instead of explicitly reading file bytes into a separate Go buffer for every access, code can access the mapped bytes directly. The operating system loads the required file pages into memory and manages writing modified pages back.

`mmap.MMap` is therefore different from `bufio.Writer`:

- `bufio.Writer`: separate memory waiting to be copied to the file.
- `mmap.MMap`: a memory-backed view of the file itself.

The operating system may still cache mapped pages in RAM. A memory mapping does not guarantee that changes have reached physical storage immediately.

### `mmap.Flush()` versus `file.Sync()`

When code modifies mapped bytes, the changes may first exist as dirty memory-mapped pages:

```text
mapped memory --mmap.Flush()--> file/OS cache --file.Sync()--> stable storage
```

- `mmap.Flush()` synchronizes changes made through the memory mapping with the file.
- `file.Sync()` synchronizes the file’s data and metadata with stable storage.

In `index.go`, `mmap.MMap` is useful because index entries have fixed sizes and can be accessed quickly using `n * entWidth`.

## Simple Summary

```text
store.go:
    stores record data

index.go:
    maps record IDs to store-file positions

bufio.Writer:
    temporary Go memory for efficient sequential writes

mmap.MMap:
    memory view of file-backed index bytes

record ID / offset:
    logical record identifier

position:
    physical byte location in the store file

entWidth:
    12 bytes, the size of one index entry
```
