package wal

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockcache"
	"github.com/ognjen-ogi/NAiSP-team-17/internal/storage/blockmanager"
)

func TestRecord(t *testing.T) {
	records := []Record{
		{
			Timestamp: 123456789,
			Tombstone: false,
			Key:       "name",
			Value:     []byte("Dmitry"),
		},
		{
			Timestamp: 987654321,
			Tombstone: true,
			Key:       "name",
			Value:     nil,
		},
	}

	for _, record := range records {
		data := serializeRecord(record)

		result, err := deserializeRecord(data)
		if err != nil {
			t.Fatal(err)
		}

		if result.Timestamp != record.Timestamp ||
			result.Tombstone != record.Tombstone ||
			result.Key != record.Key ||
			!bytes.Equal(result.Value, record.Value) {
			t.Fatal("Testiranje ser. i deser. record-a je neuspesno")
		}
	}
}

func TestFragment(t *testing.T) {
	payload := []byte("some WAL data")

	for _, fragmentType := range []FragmentType{
		FragmentFull,
		FragmentFirst,
		FragmentMiddle,
		FragmentLast,
	} {
		data, err := serializeFragment(fragmentType, payload)
		if err != nil {
			t.Fatal(err)
		}

		fragment, err := deserializeFragment(data)
		if err != nil {
			t.Fatal(err)
		}

		if fragment.Type != fragmentType ||
			fragment.Size != uint16(len(payload)) ||
			!bytes.Equal(fragment.Payload, payload) {
			t.Fatal("Testiranje ser. i deser. fragment-a je neuspesno")
		}
	}

	data, err := serializeFragment(FragmentFull, payload)
	if err != nil {
		t.Fatal(err)
	}

	data[len(data)-1]++

	if _, err = deserializeFragment(data); err == nil {
		t.Fatal("CRC validacija je neuspesna")
	}
}

func TestWALPosition(t *testing.T) {
	w := WAL{
		directory:         "data/wal",
		blockSize:         4096,
		segmentBlockCount: 2,
		currentPosition: Position{
			SegmentNumber: 1,
		},
		currentBlock: make([]byte, 4096),
	}

	if w.segmentPath(2) != filepath.Join("data/wal", "wal_0002.log") {
		t.Fatal("neispravna putanja do segmenta")
	}

	w.advanceBlock()

	if w.currentPosition.SegmentNumber != 1 ||
		w.currentPosition.BlockNumber != 1 ||
		w.currentPosition.Offset != 0 {
		t.Fatal("neispravna pozicija u bloku")
	}

	w.advanceBlock()

	if w.currentPosition.SegmentNumber != 2 ||
		w.currentPosition.BlockNumber != 0 ||
		w.currentPosition.Offset != 0 {
		t.Fatal("nismo se prebacili na drugi segment a morali smo")
	}
}

func TestWALAppend(t *testing.T) {
	const blockSize = 4096

	cache := blockcache.NewBlockCache(10)
	bm := blockmanager.NewBlockManager(blockSize, cache)

	w, err := NewWAL(t.TempDir(), bm, blockSize, 2)
	if err != nil {
		t.Fatal(err)
	}

	record := Record{
		Timestamp: 123456789,
		Key:       "key",
		Value:     bytes.Repeat([]byte("x"), 9000),
	}

	fmt.Printf("Originalna zapis: %+v\n", record)

	position, err := w.Append(record)
	if err != nil {
		t.Fatal(err)
	}

	types := []FragmentType{
		FragmentFirst,
		FragmentMiddle,
		FragmentLast,
	}

	var recordData []byte

	for i, fragmentType := range types {
		segment := 1
		block := int64(i)

		if i == 2 {
			segment = 2
			block = 0
		}

		data, err := bm.ReadBlock(w.segmentPath(segment), block)
		if err != nil {
			t.Fatal(err)
		}

		size := binary.LittleEndian.Uint16(data[4:6])

		fragment, err := deserializeFragment(
			data[:fragmentHeaderSize+int(size)],
		)

		if err != nil {
			t.Fatal(err)
		}

		if fragment.Type != fragmentType {
			t.Fatal("neispravan fragment type")
		}

		recordData = append(recordData, fragment.Payload...)
	}

	result, err := deserializeRecord(recordData)
	if err != nil {
		t.Fatal(err)
	}

	if result.Timestamp != record.Timestamp ||
		result.Key != record.Key ||
		!bytes.Equal(result.Value, record.Value) {
		t.Fatal("WAL zapisi se ne poklapaju")
	}

	if position.SegmentNumber != 2 ||
		position.BlockNumber != 0 {
		t.Fatal("nepravilna pozicija WAL")
	}

	fmt.Printf("Desereliazovana zapis: %+v\n", result)
}
