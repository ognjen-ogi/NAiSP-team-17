package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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

type segmentInfo struct {
	number int
	path   string
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

func (w *WAL) listSegments() ([]segmentInfo, error) {
	entries, err := os.ReadDir(w.directory)
	if err != nil {
		return nil, err
	}

	var segments []segmentInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasPrefix(name, "wal_") || !strings.HasSuffix(name, ".log") {
			continue
		}

		numberString := strings.TrimSuffix(strings.TrimPrefix(name, "wal_"), ".log")

		number, err := strconv.Atoi(numberString)
		if err != nil {
			continue
		}

		segments = append(segments, segmentInfo{
			number: number,
			path:   filepath.Join(w.directory, name),
		})
	}

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].number < segments[j].number
	})

	return segments, nil
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

func (w *WAL) Replay(apply func(Record) error) error {
	segments, err := w.listSegments()
	if err != nil {
		return err
	}

	var recordData []byte
	assembling := false

	for _, segment := range segments {
		info, err := os.Stat(segment.path)
		if err != nil {
			return err
		}

		blockCount := info.Size() / int64(w.blockSize)

		for blockNumber := int64(0); blockNumber < blockCount; blockNumber++ {
			block, err := w.blockManager.ReadBlock(
				segment.path,
				blockNumber,
			)

			if err != nil {
				return err
			}

			fragments, err := parseBlock(block)
			if err != nil {
				return err
			}

			for _, fragment := range fragments {
				switch fragment.Type {
				case FragmentFull:
					record, err := deserializeRecord(fragment.Payload)
					if err != nil {
						return err
					}

					if err := apply(record); err != nil {
						return err
					}

				case FragmentFirst:
					recordData = append(
						recordData[:0],
						fragment.Payload...,
					)
					assembling = true

				case FragmentMiddle:
					if !assembling {
						return fmt.Errorf("MIDDLE fragment bez FIRST fragmenta")
					}

					recordData = append(
						recordData,
						fragment.Payload...,
					)

				case FragmentLast:
					if !assembling {
						return fmt.Errorf("LAST fragment bez FIRST fragmenta")
					}

					recordData = append(
						recordData,
						fragment.Payload...,
					)

					record, err := deserializeRecord(recordData)
					if err != nil {
						return err
					}

					if err := apply(record); err != nil {
						return err
					}

					recordData = recordData[:0]
					assembling = false
				}
			}
		}
	}

	return nil
}
