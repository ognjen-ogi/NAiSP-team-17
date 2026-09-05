package blockcache

import (
	"container/list"
	"sync"
)

type cacheKey struct {
	path        string
	blockNumber int64
}
type cacheEntry struct {
	key  cacheKey
	data []byte
}
type BlockCache struct {
	mu       sync.Mutex // stiti kes od konkurentnog pristupa (vise gorutina istovremeno)
	capacity int
	list     *list.List                 // napred = najsvezije koriscen, nazad = najstariji
	items    map[cacheKey]*list.Element // brz pristup: kljuc -> cvor u listi
}

func NewBlockCache(capacity int) *BlockCache {
	return &BlockCache{
		capacity: capacity,
		list:     list.New(),
		items:    make(map[cacheKey]*list.Element),
	}
}
func (c *BlockCache) Get(path string, blockNumber int64) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey{path: path, blockNumber: blockNumber}

	element, found := c.items[key]
	if !found {
		return nil, false
	}
	c.list.MoveToFront(element)

	entry := element.Value.(*cacheEntry)
	return entry.data, true
}
func (c *BlockCache) Put(path string, blockNumber int64, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey{path: path, blockNumber: blockNumber}
	if element, found := c.items[key]; found {
		c.list.MoveToFront(element)
		entry := element.Value.(*cacheEntry)
		entry.data = data
		return
	}
	if c.list.Len() >= c.capacity {
		oldest := c.list.Back() // Back() = poslednji element = najstariji
		if oldest != nil {
			c.list.Remove(oldest)
			oldestEntry := oldest.Value.(*cacheEntry)
			delete(c.items, oldestEntry.key) // MORAMO obrisati i iz mape, ne samo iz liste
		}
	}
	newEntry := &cacheEntry{key: key, data: data}
	element := c.list.PushFront(newEntry)
	c.items[key] = element
}
func (c *BlockCache) Invalidate(path string, blockNumber int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey{path: path, blockNumber: blockNumber}
	if element, found := c.items[key]; found {
		c.list.Remove(element)
		delete(c.items, key)
	}
}

type DebugEntry struct {
	Path        string
	BlockNumber int64
	Size        int
}

func (c *BlockCache) DebugEntries() []DebugEntry {

	entries := make([]DebugEntry, 0, c.list.Len())

	for element := c.list.Front(); element != nil; element = element.Next() {
		entry := element.Value.(*cacheEntry)

		entries = append(entries, DebugEntry{
			Path:        entry.key.path,
			BlockNumber: entry.key.blockNumber,
			Size:        len(entry.data),
		})
	}

	return entries
}
