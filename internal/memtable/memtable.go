package memtable

import (
	"sort"
	"time"
)

type SizeLimitType string

const (
	SizeLimitCount SizeLimitType = "count" // limit=broj zapisa
	SizeLimitBytes SizeLimitType = "bytes" //limit=zauzece u bajtovima
)

type entry struct {
	value     []byte
	timestamp int64
	tombstone bool
}

type keyValueStore interface {
	get(key string) (entry, bool)
	put(key string, e entry)
	allSorted() []string //vraca listu kljuceva u sortiranom redosledu
	len() int
}

type hashMapStore struct {
	data map[string]entry
}

func newHashMapStore() *hashMapStore {
	return &hashMapStore{data: make(map[string]entry)}
}

func (s *hashMapStore) get(key string) (entry, bool) {
	e, found := s.data[key]
	return e, found
}

func (s *hashMapStore) put(key string, e entry) {
	s.data[key] = e
}

func (s *hashMapStore) allSorted() []string {
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *hashMapStore) len() int {
	return len(s.data)
}

type Memtable struct {
	store         keyValueStore
	sizeLimitType SizeLimitType
	sizeLimit     int
	currentBytes  int //prati trenutno zauzece u bajtovima, azurira se pri svakom Put/Delete

}
type StructureType string

const (
	StructureHashMap  StructureType = "hashmap"
	StructureSkipList StructureType = "skiplist"
	StructureBTree    StructureType = "btree"
)

func NewMemtable(sizeLimitType SizeLimitType, sizeLimit int, structureType StructureType) *Memtable {
	var store keyValueStore

	switch structureType {
	case StructureSkipList:
		store = newSkipListStore()
	case StructureBTree:
		store = newBTreeStore()
	case StructureHashMap:
		store = newHashMapStore()
	default:
		store = newHashMapStore()
	}
	return &Memtable{
		store:         store,
		sizeLimitType: sizeLimitType,
		sizeLimit:     sizeLimit,
	}
}

func (m *Memtable) Put(key string, value []byte) {
	m.PutWithTimestamp(key, value, time.Now().UnixNano())
}

func (m *Memtable) PutWithTimestamp(key string, value []byte, timestamp int64) {
	if old, found := m.store.get(key); found {
		m.currentBytes -= len(key) + len(old.value) + 9
	}

	m.store.put(key, entry{
		value:     value,
		timestamp: timestamp,
		tombstone: false,
	})

	m.currentBytes += len(key) + len(value) + 9
}
func (m *Memtable) Delete(key string) {
	m.DeleteWithTimestamp(key, time.Now().UnixNano())
}

func (m *Memtable) DeleteWithTimestamp(key string, timestamp int64) {
	if old, found := m.store.get(key); found {
		m.currentBytes -= len(key) + len(old.value) + 9
	}

	m.store.put(key, entry{
		value:     nil,
		timestamp: timestamp,
		tombstone: true,
	})

	m.currentBytes += len(key) + 9
}

func (m *Memtable) Get(key string) (value []byte, tombstone bool, found bool) {
	e, found := m.store.get(key)
	if !found {
		return nil, false, false
	}
	return e.value, e.tombstone, true
}
func (m *Memtable) IsFull() bool {
	switch m.sizeLimitType {
	case SizeLimitCount:
		return m.store.len() >= m.sizeLimit
	case SizeLimitBytes:
		return m.currentBytes >= m.sizeLimit*1024
	default:
		return false
	}
}

type FlushRecord struct {
	Key       string
	Value     []byte
	Timestamp int64
	Tombstone bool
}

func (m *Memtable) Flush() []FlushRecord {
	sortedKeys := m.store.allSorted()
	records := make([]FlushRecord, 0, len(sortedKeys))

	for _, key := range sortedKeys {
		e, _ := m.store.get(key)
		records = append(records, FlushRecord{
			Key:       key,
			Value:     e.value,
			Timestamp: e.timestamp,
			Tombstone: e.tombstone,
		})
	}
	return records
}
