package memtable

import (
	"bytes"
	"testing"
)

func TestPutAndGet(t *testing.T) {
	m := NewMemtable(SizeLimitCount, 100)

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
	m := NewMemtable(SizeLimitCount, 100)

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
	m := NewMemtable(SizeLimitCount, 100)

	_, _, found := m.Get("nepostojeci")
	if found {
		t.Fatal("Ne ocekuje se da nepostojeci kljuc bude pronadjen")
	}
}

func TestIsFullByCount(t *testing.T) {
	m := NewMemtable(SizeLimitCount, 2)

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
	m := NewMemtable(SizeLimitCount, 100)

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
