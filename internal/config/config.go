package config

type Config struct {
	WAL          WALConfig
	Memtable     MemtableConfig
	BlockManager BlockManagerConfig
	BlockCache   BlockCacheConfig
	Cache        CacheConfig
}

type WALConfig struct {
	SegmentBlockCount int64
}

type MemtableConfig struct {
	SizeLimitType string
	SizeLimit     int
	Structure     string
}

type BlockManagerConfig struct {
	BlockSize int
}

type BlockCacheConfig struct {
	Capacity int
}

type CacheConfig struct {
	Capacity int
}
