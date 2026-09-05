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
func (s *SSTable) readBloom(h tableHeader) (BloomFilter, error) {
	data, err := s.readSection(h.BloomStart, h.BloomLength)
	if err != nil {
		return BloomFilter{}, err
	}
	return deserializeBloom(data)
}
func (s *SSTable) readIndex(h tableHeader) ([]IndexEntry, error) {
	data, err := s.readSection(h.IndexStart, h.IndexLength)
	if err != nil {
		return nil, err
	}
	return deserializeIndex(data)
}
func (s *SSTable) readSummary(h tableHeader) ([]SummaryEntry, error) {
	data, err := s.readSection(h.SummaryStart, h.SummaryLength)
	if err != nil {
		return nil, err
	}
	return deserializeSummary(data)
}

func (s *SSTable) readRecord(h tableHeader, offset uint64) (Record, error) {
	lengthBytes, err := s.readAt(h.DataStart, h.DataLength, offset, 4)
	if err != nil {
		return Record{}, err
	}
	length := uint64(binary.BigEndian.Uint32(lengthBytes))
	payload, err := s.readAt(h.DataStart, h.DataLength, offset+4, length)
	if err != nil {
		return Record{}, err
	}
	records, err := parseRecords(payload)
	if err != nil || len(records) != 1 {
		return Record{}, errors.New("neispravan Data zapis")
	}
	return records[0], nil
}

func (s *SSTable) readAt(sectionStart, sectionLength, offset, length uint64) ([]byte, error) {
	if offset+length > sectionLength {
		return nil, errors.New("Data offset je van opsega")
	}
	out := make([]byte, 0, int(length))
	for length > 0 {
		blockNumber := offset / uint64(s.blockSize)
		inBlock := offset % uint64(s.blockSize)
		block, err := s.manager.ReadBlock(s.path, int64(sectionStart+blockNumber))
		if err != nil {
			return nil, err
		}
		take := uint64(len(block)) - inBlock
		if take > length {
			take = length
		}
		out = append(out, block[inBlock:inBlock+take]...)
		offset += take
		length -= take
	}
	return out, nil
}

func (s *SSTable) getLegacy(key string) ([]byte, bool, bool, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, false, nil
		}
		return nil, false, false, err
	}
	blocks := int64((info.Size() + int64(s.blockSize) - 1) / int64(s.blockSize))
	for i := int64(0); i < blocks; i++ {
		block, err := s.manager.ReadBlock(s.path, i)
		if err != nil {
			return nil, false, false, err
		}
		records, err := parseRecords(block)
		if err != nil {
			return nil, false, false, err
		}
		for _, record := range records {
			if record.Key == key {
				return record.Value, record.Tombstone, true, nil
			}
		}
	}
	return nil, false, false, nil
}

func newBloomFilter(count int) BloomFilter {
	size := count * 10
	if size < 64 {
		size = 64
	}
	return BloomFilter{Bits: make([]byte, (size+7)/8), HashCount: 4}
}
func (b *BloomFilter) Add(key string) {
	for _, position := range b.positions(key) {
		b.Bits[position/8] |= 1 << (position % 8)
	}
}
func (b BloomFilter) MightContain(key string) bool {
	for _, position := range b.positions(key) {
		if b.Bits[position/8]&(1<<(position%8)) == 0 {
			return false
		}
	}
	return true
}
func (b BloomFilter) positions(key string) []uint64 {
	if len(b.Bits) == 0 {
		return nil
	}
	result := make([]uint64, b.HashCount)
	for i := range result {
		h := fnv.New64a()
		_, _ = h.Write([]byte(fmt.Sprintf("%d:%s", i, key)))
		result[i] = h.Sum64() % uint64(len(b.Bits)*8)
	}
	return result
}
func serializeBloom(b BloomFilter) []byte {
	out := make([]byte, 4+len(b.Bits))
	binary.BigEndian.PutUint32(out[:4], b.HashCount)
	copy(out[4:], b.Bits)
	return out
}
func deserializeBloom(data []byte) (BloomFilter, error) {
	if len(data) < 4 {
		return BloomFilter{}, errors.New("neispravan Bloom Filter")
	}
	return BloomFilter{HashCount: binary.BigEndian.Uint32(data[:4]), Bits: append([]byte(nil), data[4:]...)}, nil
}

func serializeIndex(entries []IndexEntry) ([]byte, error) {
	var out bytes.Buffer
	for _, entry := range entries {
		if err := writeString(&out, entry.Key); err != nil {
			return nil, err
		}
		var offset [8]byte
		binary.BigEndian.PutUint64(offset[:], entry.Offset)
		out.Write(offset[:])
	}
	return out.Bytes(), nil
}
func deserializeIndex(data []byte) ([]IndexEntry, error) {
	var result []IndexEntry
	for len(data) > 0 {
		key, rest, err := readString(data)
		if err != nil {
			return nil, err
		}
		if len(rest) < 8 {
			return nil, errors.New("neispravan Index")
		}
		result = append(result, IndexEntry{Key: key, Offset: binary.BigEndian.Uint64(rest[:8])})
		data = rest[8:]
	}
	return result, nil
}
func serializeSummary(entries []IndexEntry, degree int) ([]byte, error) {
	var out bytes.Buffer
	for i := 0; i < len(entries); i += degree {
		if err := writeString(&out, entries[i].Key); err != nil {
			return nil, err
		}
		var offset [8]byte
		binary.BigEndian.PutUint64(offset[:], uint64(i))
		out.Write(offset[:])
	}
	if len(entries) > 0 && (len(entries)-1)%degree != 0 {
		if err := writeString(&out, entries[len(entries)-1].Key); err != nil {
			return nil, err
		}
		var offset [8]byte
		binary.BigEndian.PutUint64(offset[:], uint64(len(entries)-1))
		out.Write(offset[:])
	}
	return out.Bytes(), nil
}
func deserializeSummary(data []byte) ([]SummaryEntry, error) {
	var result []SummaryEntry
	for len(data) > 0 {
		key, rest, err := readString(data)
		if err != nil {
			return nil, err
		}
		if len(rest) < 8 {
			return nil, errors.New("neispravan Summary")
		}
		result = append(result, SummaryEntry{Key: key, IndexOffset: binary.BigEndian.Uint64(rest[:8])})
		data = rest[8:]
	}
	return result, nil
}
func writeString(out *bytes.Buffer, value string) error {
	if uint64(len(value)) > uint64(^uint32(0)) {
		return errors.New("string je predugacak")
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	out.Write(length[:])
	out.WriteString(value)
	return nil
}
func readString(data []byte) (string, []byte, error) {
	if len(data) < 4 {
		return "", nil, errors.New("neispravan string")
	}
	length := int(binary.BigEndian.Uint32(data[:4]))
	if len(data) < 4+length {
		return "", nil, errors.New("neispravan string")
	}
	return string(data[4 : 4+length]), data[4+length:], nil
}

func serializeRecord(r Record) []byte {
	keyBytes := []byte(r.Key)
	valueBytes := append([]byte(nil), r.Value...)
	var out bytes.Buffer
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(keyBytes)))
	out.Write(length[:])
	out.Write(keyBytes)
	if r.Tombstone {
		out.WriteByte(1)
	} else {
		out.WriteByte(0)
	}
	binary.BigEndian.PutUint32(length[:], uint32(len(valueBytes)))
	out.Write(length[:])
	out.Write(valueBytes)
	return out.Bytes()
}
func parseRecords(block []byte) ([]Record, error) {
	var records []Record
	for i := 0; i < len(block); {
		if i+4 > len(block) {
			break
		}
		keyLen := int(binary.BigEndian.Uint32(block[i : i+4]))
		i += 4
		if keyLen == 0 {
			break
		}
		if i+keyLen > len(block) {
			break
		}
		key := string(block[i : i+keyLen])
		i += keyLen
		if i >= len(block) {
			break
		}
		tombstone := block[i] == 1
		i++
		if i+4 > len(block) {
			break
		}
		valueLen := int(binary.BigEndian.Uint32(block[i : i+4]))
		i += 4
		if i+valueLen > len(block) {
			break
		}
		value := append([]byte(nil), block[i:i+valueLen]...)
		i += valueLen
		records = append(records, Record{Key: key, Value: value, Tombstone: tombstone})
	}
	return records, nil
}
