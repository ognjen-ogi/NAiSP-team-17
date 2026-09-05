package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	sstable "github.com/ognjen-ogi/NAiSP-team-17/internal/SSTable"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/cache"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/config"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/memtable"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockcache"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockmanager"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/wal"
)

const (
	walDirectory     = "data/wal"
	sstableDirectory = "data/sstables"
)

type Engine struct {
	config            config.Config
	wal               *wal.WAL
	memtable          *memtable.Memtable
	cache             *cache.ResultCache
	blockCache        *blockcache.BlockCache
	blockManager      *blockmanager.BlockManager
	sstables          []*sstable.SSTable
	nextSSTableNumber int
	lastWALPosition   wal.Position
}

type numberedSSTable struct {
	number int
	table  *sstable.SSTable
}

func New(configPath string) (*Engine, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("neuspesno ucitavanje konfiguracije: %w", err)
	}

	if err := os.MkdirAll(sstableDirectory, 0755); err != nil {
		return nil, fmt.Errorf("neuspesno kreiranje SSTable direktorijuma: %w", err)
	}

	blockCacheInstance := blockcache.NewBlockCache(cfg.BlockCache.Capacity)

	blockManagerInstance := blockmanager.NewBlockManager(
		cfg.BlockManager.BlockSize,
		blockCacheInstance,
	)

	walInstance, err := wal.NewWAL(
		walDirectory,
		blockManagerInstance,
		cfg.BlockManager.BlockSize,
		cfg.WAL.SegmentBlockCount,
	)
	if err != nil {
		return nil, fmt.Errorf("neuspesno kreiranje WAL-a: %w", err)
	}

	memtableInstance := memtable.NewMemtable(
		memtable.SizeLimitType(cfg.Memtable.SizeLimitType),
		cfg.Memtable.SizeLimit,
		memtable.StructureType(cfg.Memtable.Structure),
	)

	cacheInstance := cache.NewResultCache(cfg.Cache.Capacity)

	engine := &Engine{
		config:            cfg,
		wal:               walInstance,
		memtable:          memtableInstance,
		cache:             cacheInstance,
		blockCache:        blockCacheInstance,
		blockManager:      blockManagerInstance,
		sstables:          make([]*sstable.SSTable, 0),
		nextSSTableNumber: 1,
	}

	if err := engine.loadSSTables(); err != nil {
		return nil, err
	}

	if err := engine.recoverFromWAL(); err != nil {
		return nil, err
	}

	return engine, nil
}

func (e *Engine) Put(key string, value []byte) error {
	record := wal.Record{
		Timestamp: time.Now().UnixNano(),
		Tombstone: false,
		Key:       key,
		Value:     value,
	}

	position, err := e.wal.Append(record)
	if err != nil {
		return fmt.Errorf("neuspesan upis u WAL: %w", err)
	}

	e.memtable.Put(key, value)
	e.cache.Invalidate(key)
	e.lastWALPosition = position

	if e.memtable.IsFull() {
		if err := e.flushMemtable(); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) Delete(key string) error {
	record := wal.Record{
		Timestamp: time.Now().UnixNano(),
		Tombstone: true,
		Key:       key,
		Value:     nil,
	}

	position, err := e.wal.Append(record)
	if err != nil {
		return fmt.Errorf("neuspesan upis brisanja u WAL: %w", err)
	}

	e.memtable.Delete(key)
	e.cache.Invalidate(key)
	e.lastWALPosition = position

	if e.memtable.IsFull() {
		if err := e.flushMemtable(); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) Get(key string) ([]byte, bool, error) {
	value, tombstone, found := e.memtable.Get(key)
	if found {
		if tombstone {
			return nil, false, nil
		}
		return value, true, nil
	}

	value, tombstone, found = e.cache.Get(key)
	if found {
		if tombstone {
			return nil, false, nil
		}
		return value, true, nil
	}

	for i := len(e.sstables) - 1; i >= 0; i-- {
		value, tombstone, found, err := e.sstables[i].Get(key)
		if err != nil {
			return nil, false, fmt.Errorf("neuspesno citanje iz SSTable: %w", err)
		}

		if !found {
			continue
		}

		e.cache.Put(key, value, tombstone)

		if tombstone {
			return nil, false, nil
		}

		return value, true, nil
	}

	return nil, false, nil
}

func (e *Engine) flushMemtable() error {
	records := e.memtable.Flush()
	if len(records) == 0 {
		return nil
	}

	path := filepath.Join(
		sstableDirectory,
		fmt.Sprintf("sstable_%04d.db", e.nextSSTableNumber),
	)

	table := sstable.NewSSTable(
		path,
		e.config.BlockManager.BlockSize,
		e.blockCache,
	)

	if err := table.WriteRecords(records); err != nil {
		return fmt.Errorf("neuspesan flush Memtable-a u SSTable: %w", err)
	}

	e.sstables = append(e.sstables, table)
	e.nextSSTableNumber++

	e.memtable = memtable.NewMemtable(
		memtable.SizeLimitType(e.config.Memtable.SizeLimitType),
		e.config.Memtable.SizeLimit,
		memtable.StructureType(e.config.Memtable.Structure),
	)

	if e.lastWALPosition.SegmentNumber > 0 {
		if err := e.wal.SetLowWaterMark(e.lastWALPosition); err != nil {
			return fmt.Errorf("neuspesno postavljanje low-water mark pozicije: %w", err)
		}
	}

	return nil
}

func (e *Engine) loadSSTables() error {
	entries, err := os.ReadDir(sstableDirectory)
	if err != nil {
		return fmt.Errorf("neuspesno citanje SSTable direktorijuma: %w", err)
	}

	tables := make([]numberedSSTable, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasPrefix(name, "sstable_") || !strings.HasSuffix(name, ".db") {
			continue
		}

		numberText := strings.TrimSuffix(
			strings.TrimPrefix(name, "sstable_"),
			".db",
		)

		number, err := strconv.Atoi(numberText)
		if err != nil {
			continue
		}

		path := filepath.Join(sstableDirectory, name)

		table, err := sstable.Open(
			path,
			e.config.BlockManager.BlockSize,
			e.blockCache,
		)
		if err != nil {
			return fmt.Errorf("neuspesno otvaranje SSTable %s: %w", name, err)
		}

		tables = append(tables, numberedSSTable{
			number: number,
			table:  table,
		})
	}

	sort.Slice(tables, func(i, j int) bool {
		return tables[i].number < tables[j].number
	})

	for _, table := range tables {
		e.sstables = append(e.sstables, table.table)

		if table.number >= e.nextSSTableNumber {
			e.nextSSTableNumber = table.number + 1
		}
	}

	return nil
}

func (e *Engine) recoverFromWAL() error {
	err := e.wal.Replay(func(record wal.Record) error {
		if record.Tombstone {
			e.memtable.Delete(record.Key)
		} else {
			e.memtable.Put(record.Key, record.Value)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("neuspesan oporavak iz WAL-a: %w", err)
	}

	e.lastWALPosition = e.wal.CurrentPosition()

	if e.memtable.IsFull() {
		if err := e.flushMemtable(); err != nil {
			return fmt.Errorf("neuspesan flush nakon WAL recovery-ja: %w", err)
		}
	}

	return nil
}

func (e *Engine) PrintState() {
	fmt.Println()
	fmt.Println("========== STANJE ENGINE-A ==========")

	fmt.Println("MEMTABLE:")
	memtableRecords := e.memtable.Flush()

	if len(memtableRecords) == 0 {
		fmt.Println("  prazna")
	} else {
		for _, record := range memtableRecords {
			fmt.Printf(
				"  key=%s value=%s tombstone=%t\n",
				record.Key,
				string(record.Value),
				record.Tombstone,
			)
		}
	}

	fmt.Println("RESULT CACHE:")
	cacheEntries := e.cache.DebugEntries()

	if len(cacheEntries) == 0 {
		fmt.Println("  prazan")
	} else {
		for _, entry := range cacheEntries {
			fmt.Printf(
				"  key=%s value=%s tombstone=%t\n",
				entry.Key,
				string(entry.Value),
				entry.Tombstone,
			)
		}
	}

	fmt.Println("BLOCK CACHE:")
	blockEntries := e.blockCache.DebugEntries()

	if len(blockEntries) == 0 {
		fmt.Println("  prazan")
	} else {
		for _, entry := range blockEntries {
			fmt.Printf(
				"  path=%s block=%d size=%d\n",
				entry.Path,
				entry.BlockNumber,
				entry.Size,
			)
		}
	}

	fmt.Println("WAL:")
	position := e.wal.CurrentPosition()

	fmt.Printf(
		"  position: segment=%d block=%d offset=%d\n",
		position.SegmentNumber,
		position.BlockNumber,
		position.Offset,
	)

	lowWaterMark := e.wal.LowWaterMark()

	fmt.Printf(
		"  low-water mark: segment=%d block=%d offset=%d\n",
		lowWaterMark.SegmentNumber,
		lowWaterMark.BlockNumber,
		lowWaterMark.Offset,
	)

	segments, err := e.wal.SegmentNames()
	if err != nil {
		fmt.Println("  greska:", err)
	} else if len(segments) == 0 {
		fmt.Println("  nema segmenata")
	} else {
		for _, segment := range segments {
			fmt.Println(" ", segment)
		}
	}

	fmt.Println("SSTABLES:")

	if len(e.sstables) == 0 {
		fmt.Println("  nema SSTable")
	} else {
		for i, table := range e.sstables {
			fmt.Printf("  %d: %s\n", i+1, table.Path())
		}
	}

	fmt.Println("=====================================")
	fmt.Println()
}
