package wal

import (
	"fmt"
	"path/filepath"

	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockmanager"
)

type Position struct {
	SegmentNumber int
	BlockNumber   int64
	Offset        int
}

type WAL struct {
	directory         string
	blockManager      *blockmanager.BlockManager
	blockSize         int
	segmentBlockCount int64
	currentPosition   Position
	currentBlock      []byte
}

func newWAL(
	directory string,
	blockManager *blockmanager.BlockManager,
	blockSize int,
	segmentBlockCount int64,
) (*WAL, error) {
	if blockManager == nil {
		return nil, fmt.Errorf("blockmanager ne moze bitir nil")
	}

	if blockSize <= fragmentHeaderSize {
		return nil, fmt.Errorf("neispravan block size")
	}

	if segmentBlockCount <= 0 {
		return nil, fmt.Errorf("neispravan segment block count")
	}

	return &WAL{
		directory:         directory,
		blockManager:      blockManager,
		blockSize:         blockSize,
		segmentBlockCount: segmentBlockCount,
		currentPosition: Position{
			SegmentNumber: 1,
			BlockNumber:   0,
			Offset:        0,
		},
		currentBlock: make([]byte, blockSize),
	}, nil
}

func (w *WAL) segmentPath(segmentNumber int) string {
	return filepath.Join(
		w.directory,
		fmt.Sprintf("wal_%04d.log", segmentNumber),
	)
}

func (w *WAL) advanceBlock() {
	w.currentPosition.BlockNumber++
	w.currentPosition.Offset = 0
	w.currentBlock = make([]byte, w.blockSize)

	if w.currentPosition.BlockNumber >= w.segmentBlockCount {
		w.currentPosition.SegmentNumber++
		w.currentPosition.BlockNumber = 0
	}
}
