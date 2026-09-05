package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"

	"github.com/ognjen-ogi/NAiSP-team-17/internal/memtable"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockcache"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockmanager"
)

const (
	fileMagic       = "SSTABLE2"
	headerSize      = 96
	defaultSummaryN = 5
)

type Record struct {
	Key       string
	Value     []byte
	Tombstone bool
}

type BloomFilter struct {
	Bits      []byte
	HashCount uint32
}
type IndexEntry struct {
	Key    string
	Offset uint64
}
type SummaryEntry struct {
	Key         string
	IndexOffset uint64
}

type tableHeader struct {
	SummaryDegree               uint32
	RecordCount                 uint64
	DataStart, DataLength       uint64
	IndexStart, IndexLength     uint64
	SummaryStart, SummaryLength uint64
	BloomStart, BloomLength     uint64
}

type SSTable struct {
	path          string
	blockSize     int
	manager       *blockmanager.BlockManager
	summaryDegree int
}

func NewSSTable(path string, blockSize int, cache *blockcache.BlockCache) *SSTable {
	return NewSSTableWithSummaryDegree(path, blockSize, cache, defaultSummaryN)
}


func NewSSTableWithSummaryDegree(path string, blockSize int, cache *blockcache.BlockCache, degree int) *SSTable {
	if blockSize <= 0 {
		blockSize = 4096
	}
	if degree <= 0 {
		degree = defaultSummaryN
	}
	if cache == nil {
		cache = blockcache.NewBlockCache(128)
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	return &SSTable{path: path, blockSize: blockSize, manager: blockmanager.NewBlockManager(blockSize, cache), summaryDegree: degree}
}