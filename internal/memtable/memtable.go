package memtable

import "sort"

//SizeLimitType odredjuje na koji nacin se meri "punoca" Memtable-a
type SizeLimitType string

const (
	SizeLimitCount SizeLimitType = "count" // limit=broj zapisa
	SizeLimitBytes SizeLimitType = "bytes" //limit=zauzece u bajtovima
)

// entry je jedan zapis unutar Memtable-a. Cuvamo i "tombstone" flag - ako je
// true, ovaj kljuc je LOGICKI obrisan (DELETE), ali fizicki i dalje zauzima
// mesto dok se ne izvrsi kompakcija na disku. Bez ovoga bismo izgubili
// informaciju da je kljuc obrisan (Get bi mislio da kljuc nikad nije ni
// postojao, pa bi mogao da vrati staru vrednost sa diska)

type entry struct {
	value     []byte
	tombstone bool
}

// keyValueStore je apstrakcija strukture podataka ispod Memtable-a. Za ocenu 6
// postoji samo hashMapStore. Kasnije (DZ1 iz specifikacije) mozemo dodati
// skipListStore ili bTreeStore koje implementiraju isti interfejs - Memtable

type keyValueStore interface {
	get(key string) (entry, bool)
	put(key string, e entry)
	//allSorted vraca sve zapise sortirane po kljucu-potrebno za flush u
	//SSTable, koji po specifikaciji mora biti sortiran
	allSorted() []string //vraca listu kljuceva u sortiranom redosledu
	len() int
}

// hashMapStore je najjednostavnija implementacija keyValueStore - obicna Go
// mapa. Nije sortirana, pa allSorted() mora eksplicitno da sortira kljuceve
// pri svakom pozivu (to je u redu, jer se poziva samo pri flush-u, ne pri
// svakom Put/Get)

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

//Memtable je in-memory struktura koja prima najsvezije upite (PUT I DELETE)
//pre nego sto se trajno zapisu na disk kao SSTable

type Memtable struct {
	store         keyValueStore
	sizeLimitType SizeLimitType
	sizeLimit     int
	currentBytes  int //prati trenutno zauzece u bajtovima, azurira se pri svakom Put/Delete

}

// StructureType odredjuje koja konkretna struktura podataka se koristi ispod
// Memtable-a. Korisnik bira preko konfiguracije (DZ1 iz specifikacije)
type StructureType string

const (
	StructureHashMap  StructureType = "hashmap"
	StructureSkipList StructureType = "skiplist"
	//StructureBTree  ce biti dodat kada implementiramo B-stablo
)

// NewMemtable pravi novu praznu Memtable. sizeLimitType i sizeLimit dolaze iz
// konfiguracije (korisnik bira da li je limit broj elemenata ili KB).
// structureType bira konkretnu strukturu podataka ispod (DZ1)
func NewMemtable(sizeLimitType SizeLimitType, sizeLimit int, structureType StructureType) *Memtable {
	var store keyValueStore

	switch structureType {
	case StructureSkipList:
		store = newSkipListStore()
	case StructureHashMap:
		store = newHashMapStore()
	default:
		//Podrazumevano,ako konfiguracija ne navede nista-hashMapa je najjednostavnija i uvek dostupna
		store = newHashMapStore()
	}
	return &Memtable{
		store:         store,
		sizeLimitType: sizeLimitType,
		sizeLimit:     sizeLimit,
	}
}

// Put dodaje ili azurira zapis. Ovo se poziva TEK POSTO je zapis vec potvrdjen
// u WAL-u (write path: WAL prvo, pa tek onda Memtable)

func (m *Memtable) Put(key string, value []byte) {
	// Ako kljuc vec postoji, prvo "skidamo" njegovu staru velicinu iz brojaca
	// pre nego sto dodamo novu - inace bi currentBytes bio pogresan pri
	// azuriranju istog kljuca vise puta.
	if old, found := m.store.get(key); found {
		m.currentBytes -= len(key) + len(old.value)
	}

	m.store.put(key, entry{value: value, tombstone: false})
	m.currentBytes += len(key) + len(value)
}

// Delete oznacava kljuc kao logicki obrisan (tombstone), ne brise ga fizicki.
func (m *Memtable) Delete(key string) {
	if old, found := m.store.get(key); found {
		m.currentBytes -= len(key) + len(old.value)
	}

	m.store.put(key, entry{value: nil, tombstone: true})
	m.currentBytes += len(key) // tombstone zapis i dalje zauzima mesto za kljuc
}

// Get vraca vrednost za kljuc. Tri moguca ishoda:
//  1. found=true, tombstone=false  -> kljuc postoji, value je validna vrednost
//  2. found=true, tombstone=true   -> kljuc JE POSTOJAO ali je obrisan (DELETE) -
//     ovo je razlicito od "ne postoji", jer read path mora znati da NE
//     nastavlja dalje traziti kljuc u SSTable-ovima na disku
//  3. found=false                  -> Memtable uopste nema informaciju o ovom
//     kljucu, read path treba da nastavi dalje (Cache, pa SSTable-ovi)

func (m *Memtable) Get(key string) (value []byte, tombstone bool, found bool) {
	e, found := m.store.get(key)
	if !found {
		return nil, false, false
	}
	return e.value, e.tombstone, true
}

//IsFull proverava da li je Memtable dostigao konfigurisani limit
//ovo poziva engine  posle svakog Put/Delete da odluci da li treba pokrenuti flush
func (m *Memtable) IsFull() bool {
	switch m.sizeLimitType {
	case SizeLimitCount:
		return m.store.len() >= m.sizeLimit
	case SizeLimitBytes:
		// sizeLimit je u KB (po specifikaciji), currentBytes je u bajtovima.
		return m.currentBytes >= m.sizeLimit*1024
	default:
		return false
	}
}

// FlushRecord predstavlja jedan zapis spreman za upis u SSTable - sortiran po
// kljucu, sa svim informacijama koje SSTable treba (ukljucujuci tombstone,
// jer i obrisani zapisi moraju otici na disk dok kompakcija ne ocisti stare
// SSTable-ove)

type FlushRecord struct {
	Key       string
	Value     []byte
	Tombstone bool
}

//Flush vraca SVE zapise iz Memtable-a,sortirane po kljucu spremne da ih
//SSTable  komponenta zapise na disk
//Odgovornost za praznjenje je od strane engine sloja (obicno se napravi nova prazna Memtable
// a stara se zadrzi kao read-only dok flush ne zavrsi,narocito bitno DZ2)

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
