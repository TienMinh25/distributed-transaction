package wal

import (
	"errors"
	"os"
)

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

type wal struct {
	dir            string
	currentSegment *segment
	opts           Options
	lastSeq        uint64
}

func NewWAL(dir string, opts Options) (WAL, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	indexes, err := listSegmentIndexes(dir)
	if err != nil {
		return nil, err
	}

	var currentSegment *segment
	// don't find out any segment file, create new one
	if len(indexes) == 0 {
		currentSegment, err = createSegment(dir, 0)
	} else {
		currentSegment, err = openSegment(dir, indexes[len(indexes)-1])
	}
	if err != nil {
		return nil, err
	}

	w := &wal{
		dir:            dir,
		opts:           opts,
		currentSegment: currentSegment,
	}

	// after initialize, we must recover the last sequence number
	lastSeq, err := w.recoverLastSequence(dir, indexes)
	if err != nil {
		return nil, err
	}
	w.lastSeq = lastSeq

	return w, nil
}

func (w *wal) recoverLastSequence(dir string, indexes []int) (uint64, error) {
	for idx := len(indexes) - 1; idx >= 0; idx-- {
		seg, err := openSegment(dir, indexes[idx])
		if err != nil {
			return 0, err
		}
		entries, _, err := seg.readAll()
		closeErr := seg.close()
		if err != nil {
			return 0, err
		}
		if closeErr != nil {
			return 0, closeErr
		}
		if len(entries) > 0 {
			return entries[len(entries)-1].SequenceNumber, nil
		}
	}
	return 0, nil
}

func (w *wal) WriteEntry(data []byte) error {
	//TODO implement me
	panic("implement me")
}

func (w *wal) CreateCheckpoint(data []byte) error {
	//TODO implement me
	panic("implement me")
}

func (w *wal) ReadAll(fromCheckpoint bool) ([]*Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (w *wal) Sync() error {
	//TODO implement me
	panic("implement me")
}

func (w *wal) Repair() ([]*Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (w *wal) Close() error {
	return w.currentSegment.close()
}
