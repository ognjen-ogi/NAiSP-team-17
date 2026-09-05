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

func (s *SSTable) WriteRecords(records []memtable.FlushRecord) error {
	if len(records) == 0 {
		return nil
	}
	ordered := append([]memtable.FlushRecord(nil), records...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Key < ordered[j].Key })
	var data bytes.Buffer
	entries := make([]IndexEntry, 0, len(ordered))
	filter := newBloomFilter(len(ordered))
	for _, rec := range ordered {
		filter.Add(rec.Key)
		offset := uint64(data.Len())
		payload := serializeRecord(Record{Key: rec.Key, Value: rec.Value, Tombstone: rec.Tombstone})
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(payload)))
		data.Write(length[:])
		data.Write(payload)
		entries = append(entries, IndexEntry{Key: rec.Key, Offset: offset})
	}
	index, err := serializeIndex(entries)
	if err != nil {
		return err
	}
	summary, err := serializeSummary(entries, s.summaryDegree)
	if err != nil {
		return err
	}
	bloom := serializeBloom(filter)
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil && filepath.Dir(s.path) != "." {
		return err
	}
	next := uint64(1)
	dataStart, err := s.writeSection(next, data.Bytes())
	if err != nil {
		return err
	}
	next = dataStart + sectionBlocks(uint64(data.Len()), uint64(s.blockSize))
	indexStart, err := s.writeSection(next, index)
	if err != nil {
		return err
	}
	next = indexStart + sectionBlocks(uint64(len(index)), uint64(s.blockSize))
	summaryStart, err := s.writeSection(next, summary)
	if err != nil {
		return err
	}
	next = summaryStart + sectionBlocks(uint64(len(summary)), uint64(s.blockSize))
	bloomStart, err := s.writeSection(next, bloom)
	if err != nil {
		return err
	}
	header := tableHeader{SummaryDegree: uint32(s.summaryDegree), RecordCount: uint64(len(entries)), DataStart: dataStart, DataLength: uint64(data.Len()), IndexStart: indexStart, IndexLength: uint64(len(index)), SummaryStart: summaryStart, SummaryLength: uint64(len(summary)), BloomStart: bloomStart, BloomLength: uint64(len(bloom))}
	return s.manager.WriteBlock(s.path, 0, encodeHeader(s.blockSize, header))
}

func (s *SSTable) Write(records []memtable.FlushRecord) error { return s.WriteRecords(records) }

func Open(path string, blockSize int, cache *blockcache.BlockCache) (*SSTable, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return NewSSTable(path, blockSize, cache), nil
}

func (s *SSTable) Get(key string) ([]byte, bool, bool, error) {
	if s == nil {
		return nil, false, false, errors.New("sstable je nil")
	}
	h, err := s.readHeader()
	if err != nil {
		if errors.Is(err, errLegacyTable) {
			return s.getLegacy(key)
		}
		return nil, false, false, err
	}
	bloom, err := s.readBloom(h)
	if err != nil {
		return nil, false, false, err
	}
	if !bloom.MightContain(key) {
		return nil, false, false, nil
	}
	summary, err := s.readSummary(h)
	if err != nil {
		return nil, false, false, err
	}
	if len(summary) > 0 && (key < summary[0].Key || key > summary[len(summary)-1].Key) {
		return nil, false, false, nil
	}
	index, err := s.readIndex(h)
	if err != nil {
		return nil, false, false, err
	}
	start, end := 0, len(index)
	for _, entry := range summary {
		if entry.Key <= key {
			start = int(entry.IndexOffset)
			continue
		}
		end = int(entry.IndexOffset) + 1
		break
	}
	if start > end || start < 0 || end > len(index) {
		return nil, false, false, errors.New("neispravan Summary opseg")
	}
	position := start + sort.Search(end-start, func(i int) bool { return index[start+i].Key >= key })
	if position == end || index[position].Key != key {
		return nil, false, false, nil
	}
	record, err := s.readRecord(h, index[position].Offset)
	if err != nil {
		return nil, false, false, err
	}
	return record.Value, record.Tombstone, true, nil
}

func (s *SSTable) Lookup(key string) ([]byte, bool, bool, error) { return s.Get(key) }
func (s *SSTable) Read(key string) ([]byte, bool, error) {
	value, _, found, err := s.Get(key)
	return value, found, err
}
func (s *SSTable) Path() string { return s.path }
