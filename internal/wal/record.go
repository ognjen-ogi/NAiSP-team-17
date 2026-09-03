package wal

import (
	"encoding/binary"
	"fmt"
)

const recordHeaderSize = 25

type Record struct {
	Timestamp int64
	Tombstone bool
	Key       string
	Value     []byte
}

func serializeRecord(record Record) []byte {
	key := []byte(record.Key)
	value := record.Value

	if record.Tombstone {
		value = nil
	}

	data := make([]byte, recordHeaderSize+len(key)+len(value))

	binary.LittleEndian.PutUint64(data[0:8], uint64(record.Timestamp))

	if record.Tombstone {
		data[8] = 1
	}

	binary.LittleEndian.PutUint64(data[9:17], uint64(len(key)))
	binary.LittleEndian.PutUint64(data[17:25], uint64(len(value)))

	copy(data[25:25+len(key)], key)
	copy(data[25+len(key):], value)

	return data
}

func deserializeRecord(data []byte) (Record, error) {
	if len(data) < recordHeaderSize {
		return Record{}, fmt.Errorf("nepravilna duzina wal zapisa")
	}

	timestamp := int64(binary.LittleEndian.Uint64(data[0:8]))
	tombstone := data[8] != 0
	keySize := binary.LittleEndian.Uint64(data[9:17])
	valueSize := binary.LittleEndian.Uint64(data[17:25])

	payloadSize := uint64(len(data) - recordHeaderSize)

	if keySize > payloadSize || valueSize > payloadSize-keySize {
		return Record{}, fmt.Errorf("duzine key i size ne poklapaju se sa stvarnim")
	}

	if keySize+valueSize != payloadSize {
		return Record{}, fmt.Errorf("duzine key i size ne poklapaju se sa stvarnim")
	}

	keyStart := recordHeaderSize
	keyEnd := keyStart + int(keySize)
	valueEnd := keyEnd + int(valueSize)

	value := make([]byte, int(valueSize))
	copy(value, data[keyEnd:valueEnd])

	if tombstone {
		value = nil
	}

	return Record{
		Timestamp: timestamp,
		Tombstone: tombstone,
		Key:       string(data[keyStart:keyEnd]),
		Value:     value,
	}, nil
}
