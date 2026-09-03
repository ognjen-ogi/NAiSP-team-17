package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
)

const fragmentHeaderSize = 7

type FragmentType byte

const (
	FragmentFull   FragmentType = 1
	FragmentFirst  FragmentType = 2
	FragmentMiddle FragmentType = 3
	FragmentLast   FragmentType = 4
)

type Fragment struct {
	CRC     uint32
	Size    uint16
	Type    FragmentType
	Payload []byte
}

func serializeFragment(fragmentType FragmentType, payload []byte) ([]byte, error) {
	if !validFragmentType(fragmentType) {
		return nil, fmt.Errorf("neispravan type fragmenta")
	}

	if len(payload) == 0 {
		return nil, fmt.Errorf("payload fragmenta ne moze biti prazan")
	}

	if len(payload) > math.MaxUint16 {
		return nil, fmt.Errorf("payload fragmenta je prevelik")
	}

	data := make([]byte, fragmentHeaderSize+len(payload))

	crc := crc32.ChecksumIEEE(payload)

	binary.LittleEndian.PutUint32(data[0:4], crc)
	binary.LittleEndian.PutUint16(data[4:6], uint16(len(payload)))
	data[6] = byte(fragmentType)

	copy(data[7:], payload)

	return data, nil
}

func deserializeFragment(data []byte) (Fragment, error) {
	if len(data) < fragmentHeaderSize {
		return Fragment{}, fmt.Errorf("neispravan fragment")
	}

	crc := binary.LittleEndian.Uint32(data[0:4])
	size := binary.LittleEndian.Uint16(data[4:6])
	fragmentType := FragmentType(data[6])

	if size == 0 {
		return Fragment{}, fmt.Errorf("neispravna duzina payload-a fragmenta")
	}

	if !validFragmentType(fragmentType) {
		return Fragment{}, fmt.Errorf("neispravan type fragmenta")
	}

	if len(data) != fragmentHeaderSize+int(size) {
		return Fragment{}, fmt.Errorf("neispravna duzina fragmenta")
	}

	payload := make([]byte, int(size))
	copy(payload, data[fragmentHeaderSize:])

	if crc32.ChecksumIEEE(payload) != crc {
		return Fragment{}, fmt.Errorf("neispravan CRC fraqgmenta")
	}

	return Fragment{
		CRC:     crc,
		Size:    size,
		Type:    fragmentType,
		Payload: payload,
	}, nil
}

func validFragmentType(fragmentType FragmentType) bool {
	return fragmentType == FragmentFull ||
		fragmentType == FragmentFirst ||
		fragmentType == FragmentMiddle ||
		fragmentType == FragmentLast
}

func chooseFragmentType(written int, payloadSize int, totalSize int) FragmentType {
	first := written == 0
	last := written+payloadSize == totalSize

	if first && last {
		return FragmentFull
	}

	if first {
		return FragmentFirst
	}

	if last {
		return FragmentLast
	}

	return FragmentMiddle
}
