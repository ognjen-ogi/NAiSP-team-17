package cache

import (
	"bytes"
	"testing"
)

func TestResultCachePutAndGet(t *testing.T) {
	c := NewResultCache(2)

	c.Put("kljuc1", []byte("vrednost1"), false)

	value, tombstone, found := c.Get("kljuc1")
	if !found {
		t.Fatal("Ocekivano da kljuc bude pronadjen")
	}
	if tombstone {
		t.Fatal("Ne ocekuje se tombstone")
	}
	if !bytes.Equal(value, []byte("vrednost1")) {
		t.Fatal("Vrednost se ne poklapa")
	}
}

func TestResultCacheEviction(t *testing.T) {
	c := NewResultCache(2)

	c.Put("a", []byte("1"), false)
	c.Put("b", []byte("2"), false)
	c.Put("c", []byte("3"), false) // "a" treba da bude izbaceno (najstarije)

	_, _, found := c.Get("a")
	if found {
		t.Fatal("Kljuc 'a' je trebalo da bude izbacen")
	}

	_, _, found = c.Get("b")
	if !found {
		t.Fatal("Kljuc 'b' je trebalo da ostane u kesu")
	}
}

func TestResultCacheInvalidate(t *testing.T) {
	c := NewResultCache(10)

	c.Put("kljuc1", []byte("stara vrednost"), false)
	c.Invalidate("kljuc1")

	_, _, found := c.Get("kljuc1")
	if found {
		t.Fatal("Kljuc je trebalo da bude uklonjen posle Invalidate")
	}
}

func TestResultCacheTombstone(t *testing.T) {
	c := NewResultCache(10)

	c.Put("kljuc1", nil, true) // simuliramo da je GET nasao tombstone u SSTable-u

	_, tombstone, found := c.Get("kljuc1")
	if !found {
		t.Fatal("Ocekivano da kljuc bude pronadjen u kesu (kao tombstone)")
	}
	if !tombstone {
		t.Fatal("Ocekivan tombstone")
	}
}
