package memtable

import (
	"math/rand"
)

const (
	maxSkipListLevel = 16  //dovoljno za milion zapisa
	skipListP        = 0.5 //verovatnosca da element "raste" na sledeci nivo(50%)

)

// skipListNode je jedan cvor u skip listi. next je niz pokazivaca - next[0] je
// sledeci cvor na osnovnom nivou, next[1] na nivou 1, itd. Duzina next niza
// JE visina ovog konkretnog cvora (koliko nivoa "dosize")
type skipListNode struct {
	key   string
	value entry
	next  []*skipListNode
}

// skipListStore implementira keyValueStore interfejs koriscenjem skip liste.
// Za razliku od hashMapStore, ovde su podaci VEC sortirani po prirodi
// strukture, pa je allSorted() jeftiniji (nema potrebe za sort.Strings)

type skipListStore struct {
	head  *skipListNode //"prazan" pocetni cvor,ne nosi pravi kljuc
	level int           //trenutni najvisi nivo koji se koristi u listi
}

func newSkipListStore() *skipListStore {
	return &skipListStore{
		head:  &skipListNode{next: make([]*skipListNode, maxSkipListLevel)},
		level: 1,
	}
}

// randomLevel nasumicno odlucuje na koliko nivoa "raste" novi element.
// Bacamo "novcic" - 50% sanse da nastavimo na sledeci nivo, sve dok ne
// "promasimo" ili dostignemo maxSkipListLevel

func randomLevel() int {
	level := 1
	for rand.Float64() < skipListP && level < maxSkipListLevel {
		level++
	}
	return level
}

// findPredecessors nalazi, za svaki nivo, poslednji cvor CIJI je kljuc manji
// od trazenog kljuca. Ovo je "putanja" kojom smo prosli da stignemo do mesta
// gde kljuc treba da bude - koristi se i za get, i za put (ubacivanje)

func (s *skipListStore) findPredecessors(key string) []*skipListNode {
	predecessors := make([]*skipListNode, maxSkipListLevel)
	current := s.head

	// Krecemo od najviseg nivoa ka najnizem - to je sustina skip liste,
	// "preskacemo" sto vise mozemo pre nego sto sidjemo nize
	for level := s.level - 1; level >= 0; level-- {
		for current.next[level] != nil && current.next[level].key < key {
			current = current.next[level]
		}
		predecessors[level] = current
	}
	return predecessors
}

func (s *skipListStore) get(key string) (entry, bool) {
	predecessors := s.findPredecessors(key)
	//Kandidat je prvi cvor NAKON poslednjeg  predecessor-a na nivou 0
	//(osnovni nivo,koji sadrzi bas sve elemente)
	candidate := predecessors[0].next[0]

	if candidate != nil && candidate.key == key {
		return candidate.value, true
	}
	return entry{}, false
}

func (s *skipListStore) put(key string, e entry) {
	predecessors := s.findPredecessors(key)
	candidate := predecessors[0].next[0]

	//Slucaj 1:kljuc vec postoji-samo azuriramo vrednost, struktura liste(pokazivaci) ostaje ista
	if candidate != nil && candidate.key == key {
		candidate.value = e
		return
	}
	//Slucaj 2:novi-kljuc-odlucujemo nasumicno na koliko nivoa raste
	newLevel := randomLevel()

	//Ako je novi cvor "visi" nego sto je trenutni maksimalni nivo liste
	//Moramo prosiriti  listu nivoa i povezati nivoe direktno sa head-om.

	if newLevel > s.level {
		for level := s.level; level < newLevel; level++ {
			predecessors[level] = s.head
		}
		s.level = newLevel
	}
	newNode := &skipListNode{
		key:   key,
		value: e,
		next:  make([]*skipListNode, newLevel),
	}
	//Umesto novi cvor na svakom nivou do njegove visine-standardna
	//operacija umetanja u povezanu listu: novi->next=predecessor->next,
	//predecessor->next =novi
	for level := 0; level < newLevel; level++ {
		newNode.next[level] = predecessors[level].next[level]
		predecessors[level].next[level] = newNode
	}
}

// allSorted vraca kljuceve prolaskom kroz nivo 0 od pocetka do kraja-podaci su VEC sortirani
// za razliku od hashMapStore
func (s *skipListStore) allSorted() []string {
	keys := make([]string, 0)
	current := s.head.next[0]
	for current != nil {
		keys = append(keys, current.key)
		current = current.next[0]
	}
	return keys
}

func (s *skipListStore) len() int {
	count := 0
	current := s.head.next[0]
	for current != nil {
		count++
		current = current.next[0]
	}
	return count
}
