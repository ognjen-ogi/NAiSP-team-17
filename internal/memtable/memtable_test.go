package memtable

import (
	"bytes"
	"testing"
)

func TestPutAndGet(t *testing.T) {
	m := NewMemtable(SizeLimitCount, 100, StructureHashMap)

	m.Put("kljuc1", []byte("vrednost1"))

	value, tombstone, found := m.Get("kljuc1")
	if !found {
		t.Fatal("Ocekivano da kljuc bude pronadjen")
	}
	if tombstone {
		t.Fatal("Ne ocekuje se tombstone za novi zapis")
	}
	if !bytes.Equal(value, []byte("vrednost1")) {
		t.Fatal("Vrednost se ne poklapa")
	}
}

func TestDeleteCreatesTombstone(t *testing.T) {
	m := NewMemtable(SizeLimitCount, 100, StructureHashMap)

	m.Put("kljuc1", []byte("vrednost1"))
	m.Delete("kljuc1")

	value, tombstone, found := m.Get("kljuc1")
	if !found {
		t.Fatal("Ocekivano da kljuc i dalje bude 'pronadjen' (kao tombstone)")
	}
	if !tombstone {
		t.Fatal("Ocekivan tombstone posle Delete-a")
	}
	if value != nil {
		t.Fatal("Vrednost treba da bude nil za tombstone zapis")
	}
}

func TestGetNonexistentKey(t *testing.T) {
	m := NewMemtable(SizeLimitCount, 100, StructureHashMap)

	_, _, found := m.Get("nepostojeci")
	if found {
		t.Fatal("Ne ocekuje se da nepostojeci kljuc bude pronadjen")
	}
}

func TestIsFullByCount(t *testing.T) {
	m := NewMemtable(SizeLimitCount, 2, StructureHashMap)

	m.Put("a", []byte("1"))
	if m.IsFull() {
		t.Fatal("Memtable ne treba da bude pun posle 1 zapisa (limit 2)")
	}

	m.Put("b", []byte("2"))
	if !m.IsFull() {
		t.Fatal("Memtable treba da bude pun posle 2 zapisa (limit 2)")
	}
}

func TestFlushReturnsSortedRecords(t *testing.T) {
	m := NewMemtable(SizeLimitCount, 100, StructureHashMap)

	// Namerno ubacujemo van sortiranog redosleda.
	m.Put("banana", []byte("2"))
	m.Put("jabuka", []byte("1"))
	m.Put("visnja", []byte("3"))

	records := m.Flush()

	if len(records) != 3 {
		t.Fatalf("Ocekivano 3 zapisa, dobijeno %d", len(records))
	}

	expectedOrder := []string{"banana", "jabuka", "visnja"}
	for i, expectedKey := range expectedOrder {
		if records[i].Key != expectedKey {
			t.Fatalf("Na poziciji %d ocekivan kljuc %s, dobijen %s", i, expectedKey, records[i].Key)
		}
	}
}

func TestSkipListStoreBehavesLikeHashMap(t *testing.T) {
	m := NewMemtable(SizeLimitCount, 100, StructureSkipList)

	m.Put("banana", []byte("2"))
	m.Put("jabuka", []byte("1"))
	m.Put("visnja", []byte("3"))
	m.Delete("banana")

	// Proveravamo da je jabuka i dalje tu
	value, tombstone, found := m.Get("jabuka")
	if !found || tombstone || string(value) != "1" {
		t.Fatal("Skip lista: jabuka treba da postoji sa vrednoscu '1'")
	}

	// Proveravamo da je banana tombstone (obrisana)
	_, tombstone, found = m.Get("banana")
	if !found || !tombstone {
		t.Fatal("Skip lista: banana treba da bude tombstone")
	}

	// Proveravamo da je Flush i dalje sortiran
	records := m.Flush()
	if len(records) != 3 {
		t.Fatalf("Ocekivano 3 zapisa (ukljucujuci tombstone), dobijeno %d", len(records))
	}
	expectedOrder := []string{"banana", "jabuka", "visnja"}
	for i, expectedKey := range expectedOrder {
		if records[i].Key != expectedKey {
			t.Fatalf("Na poziciji %d ocekivan kljuc %s, dobijen %s", i, expectedKey, records[i].Key)
		}
	}
}

func TestBTreeStoreBehavesLikeHashMap(t *testing.T) {
	m := NewMemtable(SizeLimitCount, 100, StructureBTree)

	// Ubacujemo dovoljno kljuceva da izazovemo BAR jedan split (t=3 znaci
	// da cvor prekoracuje limit posle 5 kljuceva u istom cvoru).
	keys := []string{"k5", "k3", "k8", "k1", "k9", "k2", "k7", "k4", "k6"}
	for i, k := range keys {
		m.Put(k, []byte{byte(i)})
	}

	m.Delete("k5")

	// Proveravamo da je k3 i dalje tu.
	value, tombstone, found := m.Get("k3")
	if !found || tombstone || len(value) == 0 {
		t.Fatal("B-stablo: k3 treba da postoji")
	}

	// Proveravamo da je k5 tombstone.
	_, tombstone, found = m.Get("k5")
	if !found || !tombstone {
		t.Fatal("B-stablo: k5 treba da bude tombstone")
	}

	// Proveravamo da je Flush sortiran (kljucevi k1...k9 abecedno).
	records := m.Flush()
	if len(records) != 9 {
		t.Fatalf("Ocekivano 9 zapisa, dobijeno %d", len(records))
	}
	for i := 1; i < len(records); i++ {
		if records[i-1].Key >= records[i].Key {
			t.Fatalf("Zapisi nisu sortirani: %s pre %s", records[i-1].Key, records[i].Key)
		}
	}
}
