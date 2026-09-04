package config

const (
	DefaultWALSegmentBlockCount = 4

	DefaultMemtableSizeLimitType = "count"
	DefaultMemtableSizeLimit     = 100
	DefaultMemtableStructure     = "hashmap"

	DefaultBlockSize = 4096

	DefaultBlockCacheCapacity = 50
	DefaultCacheCapacity      = 100
)

func DefaultConfig() Config {
	return Config{
		WAL: WALConfig{
			SegmentBlockCount: DefaultWALSegmentBlockCount,
		},
		Memtable: MemtableConfig{
			SizeLimitType: DefaultMemtableSizeLimitType,
			SizeLimit:     DefaultMemtableSizeLimit,
			Structure:     DefaultMemtableStructure,
		},
		BlockManager: BlockManagerConfig{
			BlockSize: DefaultBlockSize,
		},
		BlockCache: BlockCacheConfig{
			Capacity: DefaultBlockCacheCapacity,
		},
		Cache: CacheConfig{
			Capacity: DefaultCacheCapacity,
		},
	}
}
