package memtable

// bTreeMinDegree (t) odredjuje velicinu cvorova B-stabla:
//   - maksimalan broj kljuceva po cvoru = 2t - 1
//   - minimalan broj kljuceva po cvoru (osim korena) = t - 1
// t=3 znaci: max 5 kljuceva, max 6 dece po cvoru

const bTreeMinDegree = 3

// bTreeNode je jedan cvor B-stabla. keys i values su paralelni nizovi
// (values[i] pripada keys[i]), UVEK sortirani po kljucu. Ako cvor nije list,
// children ima uvek len(keys)+1 elemenata - dete children[i] sadrzi sve
// kljuceve IZMEDJU keys[i-1] i keys[i]

type bTreeNode struct {
	keys     []string
	values   []entry
	children []*bTreeNode
	leaf     bool
}

//bTreeStore implementira keyValueStore koriscenjem B-stabla
type bTreeStore struct {
	root *bTreeNode
}

func newBTreeStore() *bTreeStore {
	return &bTreeStore{root: &bTreeNode{leaf: true}}
}

func (s *bTreeStore) get(key string) (entry, bool) {
	return searchBTree(s.root, key)
}

// searchBTree prolazi kroz cvor trazeci poziciju gde bi kljuc trebalo da
// bude. Ako ga nadje u trenutnom cvoru, vraca ga. Ako ne, i cvor nije list,
//silazi u odgovarajuce dete

func searchBTree(node *bTreeNode, key string) (entry, bool) {
	i := 0
	for i < len(node.keys) && key > node.keys[i] {
		i++
	}
	if i < len(node.keys) && key == node.keys[i] {
		return node.values[i], true
	}
	if node.leaf {
		return entry{}, false
	}
	return searchBTree(node.children[i], key)
}
func (s *bTreeStore) put(key string, e entry) {
	// Prvo proveravamo da li kljuc VEC postoji - ako da, samo azuriramo
	// vrednost na mestu, bez ikakve izmene strukture stabla
	if updateIfExists(s.root, key, e) {
		return
	}
	root := s.root
	// Ako je koren vec pun (dostigao max broj kljuceva 2t-1), MORA se
	// podeliti PRE nego sto bilo sta ubacimo. Ovo je "top-down" pristup -
	// stablo raste iskljucivo od korena naviše, nikad od listova naniže,
	// sto ga odrzava uvek balansiranim
	if len(root.keys) == 2*bTreeMinDegree-1 {
		newRoot := &bTreeNode{leaf: false, children: []*bTreeNode{root}}
		splitChild(newRoot, 0)
		s.root = newRoot
	}
	insertNonFull(s.root, key, e)
}
func updateIfExists(node *bTreeNode, key string, e entry) bool {
	i := 0
	for i < len(node.keys) && key > node.keys[i] {
		i++
	}
	if i < len(node.keys) && key == node.keys[i] {
		node.values[i] = e
		return true
	}
	if node.leaf {
		return false
	}
	return updateIfExists(node.children[i], key, e)
}

// splitChild deli PUN cvor parent.children[index] na dva cvora, i srednji
// (medijalni) kljuc "podize" kod parent-a na poziciju index. Ovo je jedina
// operacija koja menja visinu/strukturu stabla
func splitChild(parent *bTreeNode, index int) {
	t := bTreeMinDegree
	fullChild := parent.children[index]

	// Desna polovina kljuceva/vrednosti/dece ide u NOVI cvor.
	newChild := &bTreeNode{leaf: fullChild.leaf}
	newChild.keys = append(newChild.keys, fullChild.keys[t:]...)
	newChild.values = append(newChild.values, fullChild.values[t:]...)
	if !fullChild.leaf {
		newChild.children = append(newChild.children, fullChild.children[t:]...)
	}

	// Srednji (t-1-vi, 0-indeksiran) kljuc "putuje gore" kod roditelja.
	medianKey := fullChild.keys[t-1]
	medianValue := fullChild.values[t-1]

	// Stari (levi) cvor zadrzava samo prvu polovinu.
	fullChild.keys = fullChild.keys[:t-1]
	fullChild.values = fullChild.values[:t-1]
	if !fullChild.leaf {
		fullChild.children = fullChild.children[:t]
	}

	//Ubacujemo medijanu kod roditelja na tacnu poziciju(pomeramo sve postojece kljuceve/dete-pokazivace udesno da naprave mesta)
	parent.keys = append(parent.keys, "")
	copy(parent.keys[index+1:], parent.keys[index:])
	parent.keys[index] = medianKey

	parent.values = append(parent.values, entry{})
	copy(parent.values[index+1:], parent.values[index:])
	parent.values[index] = medianValue

	parent.children = append(parent.children, nil)
	copy(parent.children[index+2:], parent.children[index+1:])
	parent.children[index+1] = newChild

}

// insertNonFull umece kljuc u stablo, uz garanciju da nikad ne ulazi u vec
// PUN cvor a da ga prethodno ne podeli ("preemptivni split") - time
// izbegavamo potrebu da se posle umetanja penjemo nazad kroz stablo

func insertNonFull(node *bTreeNode, key string, e entry) {
	i := len(node.keys) - 1

	if node.leaf {
		// List - umecemo direktno, na odgovarajuce mesto da ostane sortirano.
		node.keys = append(node.keys, "")
		node.values = append(node.values, entry{})
		for i >= 0 && key < node.keys[i] {
			node.keys[i+1] = node.keys[i]
			node.values[i+1] = node.values[i]
			i--
		}
		node.keys[i+1] = key
		node.values[i+1] = e
		return
	}

	// Unutrasnji cvor - nalazimo koje dete "pokriva" ovaj kljuc.
	for i >= 0 && key < node.keys[i] {
		i--
	}
	i++

	// Ako je TO dete puno, delimo ga PRE ulaska (preemptivni split).
	if len(node.children[i].keys) == 2*bTreeMinDegree-1 {
		splitChild(node, i)
		// Posle split-a, medijana je "iskocila" kod node na poziciju i -
		// mozda sad treba da idemo u dete i+1 umesto i.
		if key > node.keys[i] {
			i++
		}
	}

	insertNonFull(node.children[i], key, e)
}

//allSorted vraca sve kljuceve in-order obilaskom(levo dete,kljuc,desno,dete...)
//ovo prirodno daje sortiran redosled kod B-stabla,kao kod binarnog stabla pretrage,samo sa vise grana po cvoru
func (s *bTreeStore) allSorted() []string {
	keys := make([]string, 0)
	inOrderKeys(s.root, &keys)
	return keys
}

func inOrderKeys(node *bTreeNode, keys *[]string) {
	for i := 0; i < len(node.keys); i++ {
		if !node.leaf {
			inOrderKeys(node.children[i], keys)
		}
		*keys = append(*keys, node.keys[i])
	}
	if !node.leaf {
		inOrderKeys(node.children[len(node.keys)], keys)
	}
}
func (s *bTreeStore) len() int {
	return len(s.allSorted())
}
