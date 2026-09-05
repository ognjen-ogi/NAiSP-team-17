package config

type Config struct {
	WAL          WALConfig          `yaml:"wal"`
	Memtable     MemtableConfig     `yaml:"memtable"`
	BlockManager BlockManagerConfig `yaml:"block_manager"`
	BlockCache   BlockCacheConfig   `yaml:"block_cache"`
	Cache        CacheConfig        `yaml:"cache"`
}

type WALConfig struct {
	SegmentBlockCount int64 `yaml:"segment_block_count"`
}

type MemtableConfig struct {
	SizeLimitType string `yaml:"size_limit_type"`
	SizeLimit     int    `yaml:"size_limit"`
	Structure     string `yaml:"structure"`
}

type BlockManagerConfig struct {
	BlockSize int `yaml:"block_size"`
}

type BlockCacheConfig struct {
	Capacity int `yaml:"capacity"`
}

type CacheConfig struct {
	Capacity int `yaml:"capacity"`
}
