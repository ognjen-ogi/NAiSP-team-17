package config

import "fmt"

func Validate(cfg Config) error {
	if cfg.WAL.SegmentBlockCount <= 0 {
		return fmt.Errorf("wal.segment_block_count mora biti veci od 0")
	}

	if cfg.Memtable.SizeLimitType != "count" &&
		cfg.Memtable.SizeLimitType != "bytes" {
		return fmt.Errorf("memtable.size_limit_type mora biti count ili bytes")
	}

	if cfg.Memtable.SizeLimit <= 0 {
		return fmt.Errorf("memtable.size_limit mora biti veci od 0")
	}

	if cfg.Memtable.Structure != "hashmap" &&
		cfg.Memtable.Structure != "skiplist" &&
		cfg.Memtable.Structure != "btree" {
		return fmt.Errorf("memtable.structure mora biti hashmap, skiplist ili btree")
	}

	if cfg.SSTable.SummaryDegree <= 0 {
		return fmt.Errorf("sstable.summary_degree mora biti veci od 0")
	}

	if cfg.BlockManager.BlockSize != 4096 &&
		cfg.BlockManager.BlockSize != 8192 &&
		cfg.BlockManager.BlockSize != 16384 {
		return fmt.Errorf("block_manager.block_size mora biti 4096, 8192 ili 16384")
	}

	if cfg.BlockCache.Capacity <= 0 {
		return fmt.Errorf("block_cache.capacity mora biti veci od 0")
	}

	if cfg.Cache.Capacity <= 0 {
		return fmt.Errorf("cache.capacity mora biti veci od 0")
	}

	return nil
}
