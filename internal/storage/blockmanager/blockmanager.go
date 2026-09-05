package blockmanager

import (
	"fmt"
	"io"
	"os"

	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockcache"
)

type BlockManager struct {
	blockSize int //velicina jednog bloka u B(4096,8192 ili 16384)
	cache     *blockcache.BlockCache
}

func NewBlockManager(blockSize int, cache *blockcache.BlockCache) *BlockManager {
	return &BlockManager{blockSize: blockSize, cache: cache}
}
func (bm *BlockManager) ReadBlock(path string, blockNumber int64) ([]byte, error) {
	if data, found := bm.cache.Get(path, blockNumber); found {
		return data, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Ne mogu da otvorim fajl %s:%w", path, err)
	}
	defer file.Close()
	offset := blockNumber * int64(bm.blockSize)
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("ne mogu da pozicioniram na offset %d: %w", offset, err)
	}
	buffer := make([]byte, bm.blockSize)
	_, err = io.ReadFull(file, buffer)
	if err != nil {
		return nil, fmt.Errorf("Ne mogu da procitam blok %d:%w", blockNumber, err)
	}

	bm.cache.Put(path, blockNumber, buffer)

	return buffer, nil

}

func (bm *BlockManager) WriteBlock(path string, blockNumber int64, data []byte) error {
	if len(data) > bm.blockSize {
		return fmt.Errorf("Podaci(%d bajtova) su veci od velicine bloka(%d bajtova)", len(data), bm.blockSize)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("ne mogu da otvorim/napravim fajl %s: %w", path, err)
	}
	defer file.Close()

	offset := blockNumber * int64(bm.blockSize)
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("ne mogu da pozicioniram na offset %d: %w", offset, err)
	}
	paddedBlock := make([]byte, bm.blockSize)
	copy(paddedBlock, data)

	_, err = file.Write(paddedBlock)
	if err != nil {
		return fmt.Errorf("Ne mogu da upišem blok %d: %w", blockNumber, err)
	}
	bm.cache.Put(path, blockNumber, paddedBlock)

	return nil
}
