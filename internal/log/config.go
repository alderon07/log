package log

type SegmentConfig struct {
		MaxStoreBytes uint64
		MaxIndexBytes uint64
		InitialOffset uint64
}

type Config struct {
	Segment SegmentConfig
}