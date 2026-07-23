package wal

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
)

/*
[8 bytes] SequenceNumber
[1 byte] IsCheckpoint
[4 bytes] len(data)
[N bytes] Data
[4 bytes] CRC
*/
func encodeEntry(e Entry) ([]byte, error) {
	buf := new(bytes.Buffer)

	// write sequence number
	if err := binary.Write(buf, binary.LittleEndian, e.SequenceNumber); err != nil {
		return nil, err
	}

	// write checkpoint
	var checkpointByte byte
	if e.IsCheckpoint {
		checkpointByte = 1
	}
	if err := buf.WriteByte(checkpointByte); err != nil {
		return nil, err
	}

	// write len(data)
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(e.Data))); err != nil {
		return nil, err
	}

	// write data
	if _, err := buf.Write(e.Data); err != nil {
		return nil, err
	}

	crc := crc32.ChecksumIEEE(buf.Bytes())
	if err := binary.Write(buf, binary.LittleEndian, crc); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decodeEntry(data []byte) (Entry, error) {
	const headerLen = 13 // 8 + 1 + 4 -> seqNo + checkpoint flag + len(data)
	const crcLen = 4

	if len(data) < headerLen+crcLen {
		return Entry{}, ErrTruncatedEntry
	}

	seqNo := binary.LittleEndian.Uint64(data[0:8])
	isCheckpoint := data[8] == 1
	dataLen := binary.LittleEndian.Uint32(data[9:13])

	dataStart := 13
	dataEnd := 13 + int(dataLen)
	if len(data) < dataEnd+crcLen {
		return Entry{}, ErrTruncatedEntry
	}

	result := data[dataStart:dataEnd]
	storedCrc := binary.LittleEndian.Uint32(data[dataEnd : dataEnd+crcLen])

	computedCrc := crc32.ChecksumIEEE(data[:dataEnd])
	if storedCrc != computedCrc {
		return Entry{}, ErrCRCMismatch
	}

	dataCopy := make([]byte, len(result))
	copy(dataCopy, result)

	return Entry{
		SequenceNumber: seqNo,
		IsCheckpoint:   isCheckpoint,
		Data:           dataCopy,
		CRC:            storedCrc,
	}, nil
}
