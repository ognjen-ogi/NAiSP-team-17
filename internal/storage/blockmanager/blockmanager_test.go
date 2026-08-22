package blockmanager

import (
	"bytes"
	"os"
	"testing"
)

func TestWriteAndReadBlock(t *testing.T) {
	// Privremeni fajl za test, briše se posle testa
	tmpFile := "test_blocks.bin"
	defer os.Remove(tmpFile)

	bm := NewBlockManager(4096)

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
