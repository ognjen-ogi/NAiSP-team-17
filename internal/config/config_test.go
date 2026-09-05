package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPartialConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	data := []byte(`
wal:
  segment_block_count: 8

memtable:
  structure: btree

block_manager:
  block_size: 8192
`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.WAL.SegmentBlockCount != 8 {
		t.Fatalf("neocekivan WAL segment_block_count")
	}

	if cfg.Memtable.Structure != "btree" {
		t.Fatalf("neocekivana Memtable structure")
	}

	if cfg.Memtable.SizeLimit != DefaultMemtableSizeLimit {
		t.Fatalf("default Memtable size_limit nije sacuvan")
	}

	if cfg.BlockManager.BlockSize != 8192 {
		t.Fatalf("neocekivan block_size")
	}

	if cfg.Cache.Capacity != DefaultCacheCapacity {
		t.Fatalf("default Cache capacity nije sacuvan")
	}
}

func TestLoadMissingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	expected := DefaultConfig()

	if cfg != expected {
		t.Fatalf("ocekivana je default konfiguracija")
	}
}

func TestInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	data := []byte(`
block_manager:
  block_size: 4097
`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("ocekivana je greska za neispravan block_size")
	}
}

func TestUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	data := []byte(`
wal:
  segment_banana: 8
`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("ocekivana je greska za nepoznato polje")
	}
}
