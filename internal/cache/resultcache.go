package cache

import (
	"container/list"
	"sync"
)

type cacheEntry struct {
	key       string
	value     []byte
	tombstone bool
}
type ResultCache struct {
	mu       sync.Mutex
	capacity int
	list     *list.List
	items    map[string]*list.Element
}

func NewResultCache(capacity int) *ResultCache {
	return &ResultCache{
		capacity: capacity,
		list:     list.New(),
		items:    make(map[string]*list.Element),
	}
}

func (c *ResultCache) Get(key string) (value []byte, tombstone bool, found bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[key]
	if !ok {
		return nil, false, false
	}

	c.list.MoveToFront(element)
	entry := element.Value.(*cacheEntry)
	return entry.value, entry.tombstone, true
}
func (c *ResultCache) Put(key string, value []byte, tombstone bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[key]; ok {
		c.list.MoveToFront(element)
		entry := element.Value.(*cacheEntry)
		entry.value = value
		entry.tombstone = tombstone
		return
	}

	if c.list.Len() >= c.capacity {
		oldest := c.list.Back()
		if oldest != nil {
			c.list.Remove(oldest)
			oldestEntry := oldest.Value.(*cacheEntry)
			delete(c.items, oldestEntry.key)
		}
	}
	newEntry := &cacheEntry{key: key, value: value, tombstone: tombstone}
	element := c.list.PushFront(newEntry)
	c.items[key] = element

}
func (c *ResultCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[key]; ok {
		c.list.Remove(element)
		delete(c.items, key)
	}
}

type DebugEntry struct {
	Key       string
	Value     []byte
	Tombstone bool
}

func (c *ResultCache) DebugEntries() []DebugEntry {

	entries := make([]DebugEntry, 0, c.list.Len())

	for element := c.list.Front(); element != nil; element = element.Next() {
		entry := element.Value.(*cacheEntry)

		entries = append(entries, DebugEntry{
			Key:       entry.key,
			Value:     append([]byte(nil), entry.value...),
			Tombstone: entry.tombstone,
		})
	}

	return entries
}
