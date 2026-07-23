package wal

import "errors"

// Entry represents for one record in WAL
type Entry struct {
	SequenceNumber uint64
	Data           []byte
	CRC            uint32 // checksum
	IsCheckpoint   bool   // used to mark this entry is checkpointed or not
}

var (
	// ErrCRCMismatch denote the entry is corrupted
	ErrCRCMismatch = errors.New("WAL: crc mismatch, entry corrupted")
	// ErrTruncatedEntry signals that there are not enough bytes to decode a complete entry.
	ErrTruncatedEntry = errors.New("wal: truncated entry")
)

type WAL interface {
	WriteEntry(data []byte) error
	CreateCheckpoint(data []byte) error
	ReadAll(fromCheckpoint bool) ([]*Entry, error)
	Sync() error
	Repair() ([]*Entry, error)
	Close() error
}
