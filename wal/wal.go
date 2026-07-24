package wal

import (
	"errors"
	"os"
	"slices"
	"sync"
)

var (
	// ErrCRCMismatch denote the entry is corrupted
	ErrCRCMismatch = errors.New("WAL: crc mismatch, entry corrupted")
	// ErrTruncatedEntry signals that there are not enough bytes to decode a complete entry.
	ErrTruncatedEntry = errors.New("WAL: truncated entry")
)

type WAL interface {
	Write(data []byte) error
	CreateCheckpoint(data []byte) error
	ReadCurrentSegment(fromCheckpoint bool) ([]Entry, error)
	Recover() ([]Entry, error)
	Sync() error
	Close() error
}

type wal struct {
	mu             sync.Mutex
	dir            string
	currentSegment *segment
	opts           Options
	lastSeq        uint64
	segmentCount   int
}

func NewWAL(dir string, opts Options) (WAL, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	indexes, err := listSegmentIndexes(dir)
	if err != nil {
		return nil, err
	}
	segmentCount := len(indexes)

	var currentSegment *segment
	// don't find out any segment file, create new one
	if segmentCount == 0 {
		currentSegment, err = createSegment(dir, 0)
		segmentCount = 1
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
		segmentCount:   segmentCount,
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
		entries, _, err := readSegment(segmentPath(dir, indexes[idx]))
		if err != nil {
			return 0, err
		}
		if len(entries) > 0 {
			return entries[len(entries)-1].SequenceNumber, nil
		}
	}
	return 0, nil
}

func (w *wal) Write(data []byte) error {
	return w.writeEntry(data, false)
}

func (w *wal) writeEntry(data []byte, isCheckpoint bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := Entry{
		SequenceNumber: w.lastSeq + 1,
		Data:           data,
		IsCheckpoint:   isCheckpoint,
	}
	encoded, err := encodeEntry(entry)
	if err != nil {
		return err
	}

	if err = w.rotateIfNeeded(int64(len(encoded) + 4)); err != nil {
		return err
	}

	err = w.currentSegment.append(encoded)
	if err != nil {
		return err
	}

	w.lastSeq++
	return nil
}

func (w *wal) rotateIfNeeded(recordSize int64) error {
	size, err := w.currentSegment.size()
	if err != nil {
		return err
	}

	if w.opts.MaxFileSize > size+recordSize {
		return nil
	}

	return w.rotate()
}

func (w *wal) rotate() error {
	if err := w.currentSegment.sync(); err != nil {
		return err
	}
	if err := w.currentSegment.close(); err != nil {
		return err
	}

	// create new segment
	newSegment, err := createSegment(w.dir, w.currentSegment.index+1)
	if err != nil {
		return err
	}
	w.currentSegment = newSegment
	w.segmentCount++

	// drop old segment if exceed the w.opts.MaxSegments
	if w.segmentCount > w.opts.MaxSegments {
		if errDel := w.deleteOldestSegment(); errDel != nil {
			return errDel
		}
		w.segmentCount--
	}

	return nil
}

func (w *wal) deleteOldestSegment() error {
	indexes, err := listSegmentIndexes(w.dir)
	if err != nil {
		return err
	}

	if len(indexes) == 0 {
		return nil
	}

	if err = os.Remove(segmentPath(w.dir, indexes[0])); err != nil {
		return err
	}

	return nil
}

func (w *wal) CreateCheckpoint(data []byte) error {
	return w.writeEntry(data, true)
}

func (w *wal) ReadCurrentSegment(fromCheckpoint bool) ([]Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	entries, checkpointSeq, err := readSegment(w.currentSegment.file.Name())
	if err != nil {
		return entries, err
	}

	if !fromCheckpoint || checkpointSeq == 0 {
		return entries, nil
	}

	idxCheckpoint := 0
	for ; idxCheckpoint < len(entries); idxCheckpoint++ {
		if entries[idxCheckpoint].SequenceNumber == checkpointSeq && entries[idxCheckpoint].IsCheckpoint {
			break
		}
	}

	result := make([]Entry, len(entries[idxCheckpoint:]))
	copy(result, entries[idxCheckpoint:])
	return result, nil
}

func (w *wal) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentSegment.sync()
}

func (w *wal) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentSegment.close()
}

func (w *wal) Recover() ([]Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	segIndexes, err := listSegmentIndexes(w.dir)
	if err != nil {
		return nil, err
	}

	var result []Entry
	found := false
	for idx := len(segIndexes) - 1; idx >= 0 && !found; idx-- {
		entries, _, errRead := readSegment(segmentPath(w.dir, segIndexes[idx]))
		if errRead != nil {
			return nil, errRead
		}

		for i := len(entries) - 1; i >= 0; i-- {
			result = append(result, entries[i])

			if entries[i].IsCheckpoint {
				found = true
				break
			}
		}
	}

	slices.Reverse(result)
	return result, nil
}
