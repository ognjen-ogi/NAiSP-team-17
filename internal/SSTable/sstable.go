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

func (s *SSTable) writeSection(start uint64, payload []byte) (uint64, error) {
	blocks := sectionBlocks(uint64(len(payload)), uint64(s.blockSize))
	if blocks == 0 {
		blocks = 1
	}
	for block := uint64(0); block < blocks; block++ {
		from := block * uint64(s.blockSize)
		to := from + uint64(s.blockSize)
		if to > uint64(len(payload)) {
			to = uint64(len(payload))
		}
		if err := s.manager.WriteBlock(s.path, int64(start+block), payload[from:to]); err != nil {
			return 0, err
		}
	}
	return start, nil
}

func sectionBlocks(length, blockSize uint64) uint64 {
	if length == 0 {
		return 0
	}
	return (length + blockSize - 1) / blockSize
}

var errLegacyTable = errors.New("legacy SSTable format")

func (s *SSTable) readHeader() (tableHeader, error) {
	block, err := s.manager.ReadBlock(s.path, 0)
	if err != nil {
		return tableHeader{}, err
	}
	if len(block) < headerSize || string(block[:len(fileMagic)]) != fileMagic {
		return tableHeader{}, errLegacyTable
	}
	return decodeHeader(block), nil
}

func encodeHeader(blockSize int, h tableHeader) []byte {
	block := make([]byte, blockSize)
	copy(block, fileMagic)
	binary.BigEndian.PutUint32(block[8:12], 1)
	binary.BigEndian.PutUint32(block[12:16], h.SummaryDegree)
	binary.BigEndian.PutUint64(block[16:24], h.RecordCount)
	values := []uint64{h.DataStart, h.DataLength, h.IndexStart, h.IndexLength, h.SummaryStart, h.SummaryLength, h.BloomStart, h.BloomLength}
	for i, value := range values {
		binary.BigEndian.PutUint64(block[24+i*8:32+i*8], value)
	}
	return block
}

func decodeHeader(block []byte) tableHeader {
	return tableHeader{SummaryDegree: binary.BigEndian.Uint32(block[12:16]), RecordCount: binary.BigEndian.Uint64(block[16:24]), DataStart: binary.BigEndian.Uint64(block[24:32]), DataLength: binary.BigEndian.Uint64(block[32:40]), IndexStart: binary.BigEndian.Uint64(block[40:48]), IndexLength: binary.BigEndian.Uint64(block[48:56]), SummaryStart: binary.BigEndian.Uint64(block[56:64]), SummaryLength: binary.BigEndian.Uint64(block[64:72]), BloomStart: binary.BigEndian.Uint64(block[72:80]), BloomLength: binary.BigEndian.Uint64(block[80:88])}
}

func (s *SSTable) readSection(start, length uint64) ([]byte, error) {
	out := make([]byte, 0, int(length))
	for block := uint64(0); uint64(len(out)) < length; block++ {
		data, err := s.manager.ReadBlock(s.path, int64(start+block))
		if err != nil {
			return nil, err
		}
		take := length - uint64(len(out))
		if take > uint64(len(data)) {
			take = uint64(len(data))
		}
		out = append(out, data[:take]...)
	}
	return out, nil
}
