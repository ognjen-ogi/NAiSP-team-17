package blockcache

import (
	"bytes"
	"testing"
)

func TestPutAndGet(t *testing.T) {
	cache := NewBlockCache(2)

	cache.Put("file.bin", 0, []byte("prvi blok"))

	data, found := cache.Get("file.bin", 0)
	if !found {
		t.Fatal("Ocekivano da blok bude pronadjen u kesu")
	}
	if !bytes.Equal(data, []byte("prvi blok")) {
		t.Fatal("Podaci se ne poklapaju")
	}
}

func TestEviction(t *testing.T) {
	// Kapacitet 2 - kad ubacimo treci blok, najstariji treba da bude izbacen.
	cache := NewBlockCache(2)

	cache.Put("file.bin", 0, []byte("A")) // kes: [0]
	cache.Put("file.bin", 1, []byte("B")) // kes: [1, 0]
	cache.Put("file.bin", 2, []byte("C")) // kes je pun (2), izbacuje se blok 0: kes: [2, 1]

	_, found := cache.Get("file.bin", 0)
	if found {
		t.Fatal("Blok 0 je trebalo da bude izbacen (najstariji), ali je i dalje u kesu")
	}

	_, found = cache.Get("file.bin", 1)
	if !found {
		t.Fatal("Blok 1 je trebalo da ostane u kesu")
	}

	_, found = cache.Get("file.bin", 2)
	if !found {
		t.Fatal("Blok 2 je trebalo da ostane u kesu")
	}
}

func TestGetRefreshesRecency(t *testing.T) {
	cache := NewBlockCache(2)

	cache.Put("file.bin", 0, []byte("A")) // kes: [0]
	cache.Put("file.bin", 1, []byte("B")) // kes: [1, 0]

	// Pristupamo bloku 0 - on postaje najsvezije koriscen: kes: [0, 1]
	cache.Get("file.bin", 0)

	// Ubacujemo blok 2 - kes je pun, treba izbaciti najstariji.
	// Posto smo malopre "osvezili" blok 0, sad je blok 1 najstariji.
	cache.Put("file.bin", 2, []byte("C")) // kes: [2, 0]

	_, found := cache.Get("file.bin", 1)
	if found {
		t.Fatal("Blok 1 je trebalo da bude izbacen jer je najduze nekoriscen")
	}

	_, found = cache.Get("file.bin", 0)
	if !found {
		t.Fatal("Blok 0 je trebalo da ostane u kesu jer je bio nedavno koriscen")
	}
}
