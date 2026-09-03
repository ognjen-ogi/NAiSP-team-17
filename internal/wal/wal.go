package wal

import (
	"fmt"
	"os"
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

func NewWAL(
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

	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
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

func (w *WAL) Append(record Record) (Position, error) {
	data := serializeRecord(record)
	written := 0

	for written < len(data) {
		remaining := w.blockSize - w.currentPosition.Offset

		if remaining < fragmentHeaderSize+1 {
			w.advanceBlock()
			remaining = w.blockSize
		}

		payloadSize := remaining - fragmentHeaderSize
		left := len(data) - written

		if left < payloadSize {
			payloadSize = left
		}

		fragmentType := chooseFragmentType(
			written,
			payloadSize,
			len(data),
		)

		fragment, err := serializeFragment(
			fragmentType,
			data[written:written+payloadSize],
		)

		if err != nil {
			return Position{}, err
		}

		copy(
			w.currentBlock[w.currentPosition.Offset:],
			fragment,
		)

		w.currentPosition.Offset += len(fragment)
		written += payloadSize

		if err := w.persistCurrentBlock(); err != nil {
			return Position{}, err
		}

		if w.currentPosition.Offset == w.blockSize ||
			w.blockSize-w.currentPosition.Offset < fragmentHeaderSize+1 {
			w.advanceBlock()
		}
	}

	return w.currentPosition, nil
}

func (w *WAL) persistCurrentBlock() error {
	return w.blockManager.WriteBlock(
		w.segmentPath(w.currentPosition.SegmentNumber),
		w.currentPosition.BlockNumber,
		w.currentBlock,
	)
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
