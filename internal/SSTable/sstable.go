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

func (s *SSTable) WriteRecords(records []memtable.FlushRecord) error {
	if len(records) == 0 {
		return nil
	}

	ordered := append([]memtable.FlushRecord(nil), records...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Key < ordered[j].Key
	})

	var payload []byte
	for _, rec := range ordered {
		payload = append(payload, serializeRecord(Record{
			Key:       rec.Key,
			Value:     rec.Value,
			Tombstone: rec.Tombstone,
		})...)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil && filepath.Dir(s.path) != "." {
		return fmt.Errorf("ne mogu da napravim direktorijum za %s: %w", s.path, err)
	}

	for idx := 0; idx < len(payload); idx += s.blockSize {
		end := idx + s.blockSize
		if end > len(payload) {
			end = len(payload)
		}

		block := make([]byte, s.blockSize)
		copy(block, payload[idx:end])
		if err := s.manager.WriteBlock(s.path, int64(idx/s.blockSize), block); err != nil {
			return fmt.Errorf("ne mogu da upisem blok %d u %s: %w", idx/s.blockSize, s.path, err)
		}
	}

	return nil
}

func (s *SSTable) Write(records []memtable.FlushRecord) error {
	return s.WriteRecords(records)
}