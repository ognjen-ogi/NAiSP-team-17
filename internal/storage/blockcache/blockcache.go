package blockcache

import (
	"container/list"
	"sync"
)

// cacheKey jedinstveno identifikuje jedan blok - kombinacija putanje do fajla
// i rednog broja bloka unutar tog fajla (isti broj bloka u DVA razlicita fajla
// je DVA razlicita bloka).
type cacheKey struct {
	path        string
	blockNumber int64
}

// cacheEntry je ono sto se stvarno cuva u cvoru liste - kljuc (da bismo mogli
// da ga obrisemo iz mape kad ga izbacujemo iz liste) i sami podaci bloka.
type cacheEntry struct {
	key  cacheKey
	data []byte
}

// BlockCache je LRU kes za blokove. Kombinuje hash mapu (brz pristup) i
// dvostruko povezanu listu (pracenje redosleda koriscenja).
type BlockCache struct {
	mu       sync.Mutex // stiti kes od konkurentnog pristupa (vise gorutina istovremeno)
	capacity int
	list     *list.List                 // napred = najsvezije koriscen, nazad = najstariji
	items    map[cacheKey]*list.Element // brz pristup: kljuc -> cvor u listi
}

// NewBlockCache pravi novi LRU kes sa zadatim kapacitetom (broj blokova, ne bajtova).
func NewBlockCache(capacity int) *BlockCache {
	return &BlockCache{
		capacity: capacity,
		list:     list.New(),
		items:    make(map[cacheKey]*list.Element),
	}
}

// Get vraca sadrzaj bloka ako je u kesu (hit), i pomera ga na pocetak liste
// (postaje "najsvezije koriscen"). Drugi povratni parametar govori da li je
// blok pronadjen (isto kao "value, ok := map[key]" pattern u Go-u).
func (c *BlockCache) Get(path string, blockNumber int64) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey{path: path, blockNumber: blockNumber}

	element, found := c.items[key]
	if !found {
		return nil, false
	}

	// MoveToFront pomera POSTOJECI cvor na pocetak liste - O(1) operacija,
	// jer dvostruko povezana lista zna direktno prethodnika/sledbenika cvora.
	c.list.MoveToFront(element)

	entry := element.Value.(*cacheEntry)
	return entry.data, true
}

// Put ubacuje ili azurira blok u kesu. Ako je kes pun, izbacuje se najstariji
// (najduze nekoriscen) blok da napravi mesta.
func (c *BlockCache) Put(path string, blockNumber int64, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey{path: path, blockNumber: blockNumber}

	// Slucaj 1: blok VEC postoji u kesu - samo azuriramo podatke i
	// pomeramo na pocetak (postao je najsvezije koriscen).
	if element, found := c.items[key]; found {
		c.list.MoveToFront(element)
		entry := element.Value.(*cacheEntry)
		entry.data = data
		return
	}

	// Slucaj 2: kes je PUN - moramo izbaciti najstariji element pre nego
	// sto ubacimo novi.
	if c.list.Len() >= c.capacity {
		oldest := c.list.Back() // Back() = poslednji element = najstariji
		if oldest != nil {
			c.list.Remove(oldest)
			oldestEntry := oldest.Value.(*cacheEntry)
			delete(c.items, oldestEntry.key) // MORAMO obrisati i iz mape, ne samo iz liste
		}
	}

	// Slucaj 3: ubacujemo novi element na pocetak liste (najsvezije koriscen).
	newEntry := &cacheEntry{key: key, data: data}
	element := c.list.PushFront(newEntry)
	c.items[key] = element
}

// Invalidate uklanja blok iz kesa, ako postoji. Korisno npr. kad znamo da je
// fajl obrisan ili prepisan na nacin koji zaobilazi normalan Put.
func (c *BlockCache) Invalidate(path string, blockNumber int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey{path: path, blockNumber: blockNumber}
	if element, found := c.items[key]; found {
		c.list.Remove(element)
		delete(c.items, key)
	}
}
