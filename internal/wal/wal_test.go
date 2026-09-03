package wal

import (
	"bytes"
	"testing"
)

func TestSerializeDeserializeRecord(t *testing.T) {
	record := Record{
		Timestamp: 123456789,
		Tombstone: false,
		Key:       "name",
		Value:     []byte("Dmitry"),
	}

	data := serializeRecord(record)

	result, err := deserializeRecord(data)
	if err != nil {
		t.Fatal(err)
	}

	if result.Timestamp != record.Timestamp {
		t.Errorf("ocekivan timestamp %d, dobili %d", record.Timestamp, result.Timestamp)
	}

	if result.Tombstone != record.Tombstone {
		t.Errorf("ocekivan tombstone %v, dobili %v", record.Tombstone, result.Tombstone)
	}

	if result.Key != record.Key {
		t.Errorf("ocekivan key %s, dobili %s", record.Key, result.Key)
	}

	if !bytes.Equal(result.Value, record.Value) {
		t.Errorf("ocekivan value %v, dobili %v", record.Value, result.Value)
	}
}

func TestSerializeDeserializeTombstone(t *testing.T) {
	record := Record{
		Timestamp: 987654321,
		Tombstone: true,
		Key:       "name",
		Value:     nil,
	}

	data := serializeRecord(record)

	result, err := deserializeRecord(data)
	if err != nil {
		t.Fatal(err)
	}

	if result.Timestamp != record.Timestamp {
		t.Errorf("ocekuivan %d, dobili %d", record.Timestamp, result.Timestamp)
	}

	if !result.Tombstone {
		t.Error(" tombstone trebalo je da bude true")
	}

	if result.Key != record.Key {
		t.Errorf("ocekuivan key %s, dobili %s", record.Key, result.Key)
	}

	if result.Value != nil {
		t.Errorf("ocekuivan nil value, dobili %v", result.Value)
	}
}

func TestSerializeDeserializeFragment(t *testing.T) {
	payload := []byte("wal data")

	data, err := serializeFragment(FragmentFull, payload)
	if err != nil {
		t.Fatal(err)
	}

	fragment, err := deserializeFragment(data)
	if err != nil {
		t.Fatal(err)
	}

	if fragment.Type != FragmentFull {
		t.Errorf("ocekivan fragment type %d, dobili %d", FragmentFull, fragment.Type)
	}

	if fragment.Size != uint16(len(payload)) {
		t.Errorf("ocekivan size %d, dobili %d", len(payload), fragment.Size)
	}

	if !bytes.Equal(fragment.Payload, payload) {
		t.Errorf("ocekivan payload %v, dobili %v", payload, fragment.Payload)
	}
}

func TestFragmentCRC(t *testing.T) {
	payload := []byte("wal data")

	data, err := serializeFragment(FragmentFull, payload)
	if err != nil {
		t.Fatal(err)
	}

	data[len(data)-1]++

	_, err = deserializeFragment(data)
	if err == nil {
		t.Fatal("ocekivana greska CRC")
	}
}

func TestFragmentTypes(t *testing.T) {
	types := []FragmentType{
		FragmentFull,
		FragmentFirst,
		FragmentMiddle,
		FragmentLast,
	}

	for _, fragmentType := range types {
		data, err := serializeFragment(fragmentType, []byte("data"))
		if err != nil {
			t.Fatal(err)
		}

		fragment, err := deserializeFragment(data)
		if err != nil {
			t.Fatal(err)
		}

		if fragment.Type != fragmentType {
			t.Errorf("ocekivan fragment type %d, dobili %d", fragmentType, fragment.Type)
		}
	}
}
