package config

const (
	DefaultWALSegmentBlockCount = 4

	DefaultMemtableSizeLimitType = "count"
	DefaultMemtableSizeLimit     = 100
	DefaultMemtableStructure     = "hashmap"

	DefaultSummaryDegree = 5

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
		SSTable: SSTableConfig{
			SummaryDegree: DefaultSummaryDegree,
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
