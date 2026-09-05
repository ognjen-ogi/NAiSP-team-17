package memtable

import (
	"math/rand"
)

const (
	maxSkipListLevel = 16  //dovoljno za milion zapisa
	skipListP        = 0.5 //verovatnosca da element "raste" na sledeci nivo(50%)

)

type skipListNode struct {
	key   string
	value entry
	next  []*skipListNode
}

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

func randomLevel() int {
	level := 1
	for rand.Float64() < skipListP && level < maxSkipListLevel {
		level++
	}
	return level
}

func (s *skipListStore) findPredecessors(key string) []*skipListNode {
	predecessors := make([]*skipListNode, maxSkipListLevel)
	current := s.head
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
	candidate := predecessors[0].next[0]

	if candidate != nil && candidate.key == key {
		return candidate.value, true
	}
	return entry{}, false
}

func (s *skipListStore) put(key string, e entry) {
	predecessors := s.findPredecessors(key)
	candidate := predecessors[0].next[0]
	if candidate != nil && candidate.key == key {
		candidate.value = e
		return
	}
	newLevel := randomLevel()

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
	for level := 0; level < newLevel; level++ {
		newNode.next[level] = predecessors[level].next[level]
		predecessors[level].next[level] = newNode
	}
}
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
