package blockmanager

import (
	"bytes"
	"os"
	"testing"

	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockcache"
)

func TestWriteAndReadBlock(t *testing.T) {
	// Privremeni fajl za test, briše se posle testa
	tmpFile := "test_blocks.bin"
	defer os.Remove(tmpFile)

	cache := blockcache.NewBlockCache(10) //10 blokova kapaciteta za test
	bm := NewBlockManager(4096, cache)

	original := []byte("Zdravo, ovo je test bloka!")

	err := bm.WriteBlock(tmpFile, 0, original)
	if err != nil {
		t.Fatalf("WriteBlock je vratio grešku: %v", err)
	}

	readBack, err := bm.ReadBlock(tmpFile, 0)
	if err != nil {
		t.Fatalf("ReadBlock je vratio grešku: %v", err)
	}

	// readBack je ceo blok (4096 bajtova, sa paddingom), original je kraći,
	// zato poredimo samo prvih len(original) bajtova.
	if !bytes.Equal(readBack[:len(original)], original) {
		t.Fatalf("Pročitani podaci se ne poklapaju sa upisanim")
	}
}
