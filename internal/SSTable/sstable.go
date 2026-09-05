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

func Open(path string, blockSize int, cache *blockcache.BlockCache) (*SSTable, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return NewSSTable(path, blockSize, cache), nil
}

// returns value, tombstone, found, error.
func (s *SSTable) Get(key string) ([]byte, bool, bool, error) {
	if s == nil {
		return nil, false, false, errors.New("sstable je nil")
	}

	info, err := os.Stat(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, false, nil
		}
		return nil, false, false, fmt.Errorf("ne mogu da procitam stat za %s: %w", s.path, err)
	}
	if info.Size() == 0 {
		return nil, false, false, nil
	}

	blocks := int64((info.Size() + int64(s.blockSize) - 1) / int64(s.blockSize))
	for i := int64(0); i < blocks; i++ {
		block, err := s.manager.ReadBlock(s.path, i)
		if err != nil {
			return nil, false, false, fmt.Errorf("ne mogu da procitam blok %d: %w", i, err)
		}
		records, err := parseRecords(block)
		if err != nil {
			return nil, false, false, fmt.Errorf("ne mogu da parsiram blok %d: %w", i, err)
		}
		for _, rec := range records {
			if rec.Key == key {
				return rec.Value, rec.Tombstone, true, nil
			}
		}
	}

	return nil, false, false, nil
}

// Get alias
func (s *SSTable) Lookup(key string) ([]byte, bool, bool, error) {
	return s.Get(key)
}

// returns value, found
func (s *SSTable) Read(key string) ([]byte, bool, error) {
	value, _, found, err := s.Get(key)
	return value, found, err
}