package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ognjen-ogi/NAiSP-team-17/internal/memtable"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockcache"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockmanager"
)


type Record struct {
	Key       string
	Value     []byte
	Tombstone bool
}

type SSTable struct {
	path      string
	blockSize int
	manager   *blockmanager.BlockManager
}

func NewSSTable(path string, blockSize int, cache *blockcache.BlockCache) *SSTable {
	if blockSize <= 0 {
		blockSize = 4096
	}
	if cache == nil {
		cache = blockcache.NewBlockCache(128)
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	return &SSTable{
		path:      path,
		blockSize: blockSize,
		manager:   blockmanager.NewBlockManager(blockSize, cache),
	}
}