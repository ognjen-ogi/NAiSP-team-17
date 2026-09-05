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

const lowWaterMarkFileName = "lwm.meta"

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
	lowWaterMark      Position
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

	w := &WAL{
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
		lowWaterMark: Position{
			SegmentNumber: 1,
			BlockNumber:   0,
			Offset:        0,
		},
	}

	lowWaterMark, err := w.loadLowWaterMark()
	if err != nil {
		return nil, err
	}

	w.lowWaterMark = lowWaterMark

	if err := w.restoreWritePosition(); err != nil {
		return nil, err
	}

	if positionBefore(w.currentPosition, w.lowWaterMark) {
		if w.lowWaterMark.BlockNumber != 0 || w.lowWaterMark.Offset != 0 {
			return nil, fmt.Errorf("WAL pozicija je pre low-water mark pozicije")
		}

		w.currentPosition = w.lowWaterMark
		w.currentBlock = make([]byte, blockSize)
	}

	return w, nil
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

	skipOrphanFragments := len(segments) > 0 &&
		segments[0].number > w.lowWaterMark.SegmentNumber

	for _, segment := range segments {
		if segment.number < w.lowWaterMark.SegmentNumber {
			continue
		}

		info, err := os.Stat(segment.path)
		if err != nil {
			return err
		}

		blockCount := info.Size() / int64(w.blockSize)

		startBlock := int64(0)

		if segment.number == w.lowWaterMark.SegmentNumber {
			startBlock = w.lowWaterMark.BlockNumber
		}

		for blockNumber := startBlock; blockNumber < blockCount; blockNumber++ {
			block, err := w.blockManager.ReadBlock(
				segment.path,
				blockNumber,
			)
			if err != nil {
				return err
			}

			startOffset := 0

			if segment.number == w.lowWaterMark.SegmentNumber &&
				blockNumber == w.lowWaterMark.BlockNumber {
				startOffset = w.lowWaterMark.Offset
			}

			if startOffset < 0 || startOffset > len(block) {
				return fmt.Errorf("neispravan low-water mark offset")
			}

			fragments, err := parseBlock(block[startOffset:])
			if err != nil {
				return err
			}

			for _, fragment := range fragments {
				switch fragment.Type {
				case FragmentFull:
					if assembling {
						recordData = recordData[:0]
						assembling = false
					}

					skipOrphanFragments = false

					record, err := deserializeRecord(fragment.Payload)
					if err != nil {
						return err
					}

					if err := apply(record); err != nil {
						return err
					}

				case FragmentFirst:
					if assembling {
						recordData = recordData[:0]
					}

					skipOrphanFragments = false
					recordData = append(recordData[:0], fragment.Payload...)
					assembling = true

				case FragmentMiddle:
					if !assembling {
						if skipOrphanFragments {
							continue
						}

						return fmt.Errorf("MIDDLE fragment bez FIRST fragmenta")
					}

					recordData = append(recordData, fragment.Payload...)

				case FragmentLast:
					if !assembling {
						if skipOrphanFragments {
							skipOrphanFragments = false
							continue
						}

						return fmt.Errorf("LAST fragment bez FIRST fragmenta")
					}

					recordData = append(recordData, fragment.Payload...)

					record, err := deserializeRecord(recordData)
					if err != nil {
						return err
					}

					if err := apply(record); err != nil {
						return err
					}

					recordData = recordData[:0]
					assembling = false
					skipOrphanFragments = false
				}
			}
		}
	}

	return nil
}

func (w *WAL) restoreWritePosition() error {
	segments, err := w.listSegments()
	if err != nil {
		return err
	}

	if len(segments) == 0 {
		return nil
	}

	lastSegment := segments[len(segments)-1]

	info, err := os.Stat(lastSegment.path)
	if err != nil {
		return err
	}

	if info.Size()%int64(w.blockSize) != 0 {
		return fmt.Errorf("velicina WAL segmenta nije deljiva sa block size")
	}

	blockCount := info.Size() / int64(w.blockSize)

	if blockCount > w.segmentBlockCount {
		return fmt.Errorf("WAL segment ima vise blokova nego stoo je dozvoljeno")
	}

	if blockCount == 0 {
		w.currentPosition.SegmentNumber = lastSegment.number
		return nil
	}

	lastBlockNumber := blockCount - 1

	block, err := w.blockManager.ReadBlock(
		lastSegment.path,
		lastBlockNumber,
	)
	if err != nil {
		return err
	}

	fragments, err := parseBlock(block)
	if err != nil {
		return err
	}

	used := 0

	for _, fragment := range fragments {
		used += fragmentHeaderSize + int(fragment.Size)
	}

	w.currentPosition = Position{
		SegmentNumber: lastSegment.number,
		BlockNumber:   lastBlockNumber,
		Offset:        used,
	}

	w.currentBlock = append([]byte(nil), block...)

	if used == w.blockSize ||
		w.blockSize-used < fragmentHeaderSize+1 {
		w.advanceBlock()
	}

	return nil
}

func (w *WAL) SetLowWaterMark(lowWaterMark Position) error {
	if !w.validPosition(lowWaterMark) {
		return fmt.Errorf("neispravna low-water mark pozicija")
	}

	if positionBefore(w.currentPosition, lowWaterMark) {
		return fmt.Errorf("low-water mark ne moze biti posle trenutne WAL pozicije")
	}

	if positionBefore(lowWaterMark, w.lowWaterMark) {
		return fmt.Errorf("low-water mark ne moze da se pomera unazad")
	}

	if err := w.saveLowWaterMark(lowWaterMark); err != nil {
		return err
	}

	w.lowWaterMark = lowWaterMark

	segments, err := w.listSegments()
	if err != nil {
		return err
	}

	for _, segment := range segments {
		if segment.number >= lowWaterMark.SegmentNumber {
			continue
		}

		if err := os.Remove(segment.path); err != nil {
			return err
		}
	}

	return nil
}

func (w *WAL) CurrentPosition() Position {
	return w.currentPosition
}

func (w *WAL) SegmentNames() ([]string, error) {
	segments, err := w.listSegments()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(segments))

	for _, segment := range segments {
		names = append(names, filepath.Base(segment.path))
	}

	return names, nil
}

func (w *WAL) lowWaterMarkPath() string {
	return filepath.Join(w.directory, lowWaterMarkFileName)
}

func (w *WAL) saveLowWaterMark(position Position) error {
	data := []byte(fmt.Sprintf(
		"%d %d %d\n",
		position.SegmentNumber,
		position.BlockNumber,
		position.Offset,
	))

	if err := w.blockManager.WriteBlock(
		w.lowWaterMarkPath(),
		0,
		data,
	); err != nil {
		return fmt.Errorf("neuspesno cuvanje low-water mark pozicije: %w", err)
	}

	return nil
}

func (w *WAL) loadLowWaterMark() (Position, error) {
	defaultPosition := Position{
		SegmentNumber: 1,
		BlockNumber:   0,
		Offset:        0,
	}

	_, err := os.Stat(w.lowWaterMarkPath())
	if os.IsNotExist(err) {
		return defaultPosition, nil
	}

	if err != nil {
		return Position{}, fmt.Errorf("neuspesna provera low-water mark fajla: %w", err)
	}

	data, err := w.blockManager.ReadBlock(
		w.lowWaterMarkPath(),
		0,
	)
	if err != nil {
		return Position{}, fmt.Errorf("neuspesno citanje low-water mark pozicije: %w", err)
	}

	var position Position

	_, err = fmt.Sscanf(
		string(data),
		"%d %d %d",
		&position.SegmentNumber,
		&position.BlockNumber,
		&position.Offset,
	)
	if err != nil {
		return Position{}, fmt.Errorf("neispravna low-water mark pozicija: %w", err)
	}

	if !w.validPosition(position) {
		return Position{}, fmt.Errorf("neispravna low-water mark pozicija")
	}

	return position, nil
}

func (w *WAL) validPosition(position Position) bool {
	return position.SegmentNumber >= 1 &&
		position.BlockNumber >= 0 &&
		position.BlockNumber < w.segmentBlockCount &&
		position.Offset >= 0 &&
		position.Offset < w.blockSize
}

func positionBefore(first Position, second Position) bool {
	if first.SegmentNumber != second.SegmentNumber {
		return first.SegmentNumber < second.SegmentNumber
	}

	if first.BlockNumber != second.BlockNumber {
		return first.BlockNumber < second.BlockNumber
	}

	return first.Offset < second.Offset
}

func (w *WAL) LowWaterMark() Position {
	return w.lowWaterMark
}
