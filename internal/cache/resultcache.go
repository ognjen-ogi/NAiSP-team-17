package cache

import (
	"container/list"
	"sync"
)

// cacheEntry je ono sto se cuva u cvoru liste-kljuc(da bismo mogli da ga obrisemo iz mape kad ga izbacimo iz liste) i sama vrednost
type cacheEntry struct {
	key       string
	value     []byte
	tombstone bool
}

// ResultCache je LRU kes za rezultate GET operacija(1.5 iz specifikacije)
// Razlikuje se od Block Cache-a(1.6) po tome sto ovde kljuc nije blok sa diska
// vec pravi korisnicki kljuc, a vrednost  je direktno ono sto GET treba da vrati
type ResultCache struct {
	mu       sync.Mutex
	capacity int
	list     *list.List
	items    map[string]*list.Element
}

// NewResultCache pravi novi LRU kes sa zadatim kapacitetom(broj zapisa)
// Kapacitet dolazi iz konfiguracije-korisnik ga bira
func NewResultCache(capacity int) *ResultCache {
	return &ResultCache{
		capacity: capacity,
		list:     list.New(),
		items:    make(map[string]*list.Element),
	}
}

//Get vraca vrednost za kljuc ako je u kesu(hit), i pomera ga na pocetak liste(postaje najsvezije  koriscen)
//tombstone govori da li je ovo zapamceno kao "obrisan kljuc"(DELETE)- vazno da GET ne vrati pogresno
//"ne postoji" umesto "eksplicitno obrisan"

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

// Put ubacuje ili azurira rezultat u kesu. Poziva se kad GET pronadje vrednost negde dublje(SSTable) i zeli da je zapamti za sledeci put
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

// Invalidate uklanja kljuc iz kesa, ako postoji
// Mora se pozvati kad god se kljuc menja ili brise negde "ispod kesa"(npr. PUT ili DELETE u write path-u)
// da ne ostane sa zastarelom vrednoscu
func (c *ResultCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[key]; ok {
		c.list.Remove(element)
		delete(c.items, key)
	}
}
