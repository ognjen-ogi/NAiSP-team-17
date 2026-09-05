package memtable

const bTreeMinDegree = 3

type bTreeNode struct {
	keys     []string
	values   []entry
	children []*bTreeNode
	leaf     bool
}
type bTreeStore struct {
	root *bTreeNode
}

func newBTreeStore() *bTreeStore {
	return &bTreeStore{root: &bTreeNode{leaf: true}}
}

func (s *bTreeStore) get(key string) (entry, bool) {
	return searchBTree(s.root, key)
}

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
	if updateIfExists(s.root, key, e) {
		return
	}
	root := s.root
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
func splitChild(parent *bTreeNode, index int) {
	t := bTreeMinDegree
	fullChild := parent.children[index]
	newChild := &bTreeNode{leaf: fullChild.leaf}
	newChild.keys = append(newChild.keys, fullChild.keys[t:]...)
	newChild.values = append(newChild.values, fullChild.values[t:]...)
	if !fullChild.leaf {
		newChild.children = append(newChild.children, fullChild.children[t:]...)
	}
	medianKey := fullChild.keys[t-1]
	medianValue := fullChild.values[t-1]
	fullChild.keys = fullChild.keys[:t-1]
	fullChild.values = fullChild.values[:t-1]
	if !fullChild.leaf {
		fullChild.children = fullChild.children[:t]
	}
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

func insertNonFull(node *bTreeNode, key string, e entry) {
	i := len(node.keys) - 1

	if node.leaf {
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
	for i >= 0 && key < node.keys[i] {
		i--
	}
	i++
	if len(node.children[i].keys) == 2*bTreeMinDegree-1 {
		splitChild(node, i)
		if key > node.keys[i] {
			i++
		}
	}

	insertNonFull(node.children[i], key, e)
}
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
