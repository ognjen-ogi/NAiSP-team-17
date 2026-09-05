package memtable

import "sort"

type SizeLimitType string

const (
	SizeLimitCount SizeLimitType = "count" // limit=broj zapisa
	SizeLimitBytes SizeLimitType = "bytes" //limit=zauzece u bajtovima
)

type entry struct {
	value     []byte
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
	if old, found := m.store.get(key); found {
		m.currentBytes -= len(key) + len(old.value)
	}

	m.store.put(key, entry{value: value, tombstone: false})
	m.currentBytes += len(key) + len(value)
}
func (m *Memtable) Delete(key string) {
	if old, found := m.store.get(key); found {
		m.currentBytes -= len(key) + len(old.value)
	}

	m.store.put(key, entry{value: nil, tombstone: true})
	m.currentBytes += len(key) // tombstone zapis i dalje zauzima mesto za kljuc
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
			Tombstone: e.tombstone,
		})
	}
	return records
}
