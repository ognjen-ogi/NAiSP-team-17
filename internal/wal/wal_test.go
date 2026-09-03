package wal

import (
	"bytes"
	"path/filepath"
	"testing"
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
