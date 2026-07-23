package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

/*
- we split the log file to many smaller log file (which is called segment)
- we wanna improve write performance, so instead of writing directly data to disk and flush, we \
will write it to in-memory buffer and flush these buffer to disk at set intervals.
*/
type segment struct {
	file      *os.File      // segment file
	bufWriter *bufio.Writer // buffer
	index     int           // segment index
}

func segmentPath(dir string, index int) string {
	return filepath.Join(dir, fmt.Sprintf("%s%d.wal", SEGMENT_PREFIX, index))
}

// createSegment create new segment file, return error if it's already exists
// create in append and write only mode
func createSegment(dir string, index int) (*segment, error) {
	path := segmentPath(dir, index)
	fd, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &segment{
		file:      fd,
		bufWriter: bufio.NewWriter(fd),
		index:     index,
	}, nil
}

// openSegment open one file in mode append and already seek to the end file
func openSegment(dir string, index int) (*segment, error) {
	path := segmentPath(dir, index)
	fd, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	if _, err = fd.Seek(0, io.SeekEnd); err != nil {
		fd.Close()
		return nil, err
	}

	return &segment{
		file:      fd,
		bufWriter: bufio.NewWriter(fd),
		index:     index,
	}, nil
}

// append add entry to the buffer (it's not write to file yet, if you need to write file immediately, use sync() func)
// the record will be written like format: length-prefix of entry + entry
func (s *segment) append(encoded []byte) error {
	if err := binary.Write(s.bufWriter, binary.LittleEndian, uint32(len(encoded))); err != nil {
		return err
	}
	_, err := s.bufWriter.Write(encoded)

	return err
}

// close release resource
func (s *segment) close() error {
	// after flush, the data is in kernel memory, then it will be
	// written to file, no need to call s.file.Sync()
	_ = s.bufWriter.Flush()
	return s.file.Close()
}

// size return the total bytes in both segment file and buffers
func (s *segment) size() (int64, error) {
	fileInfo, err := s.file.Stat()
	if err != nil {
		return 0, err
	}

	return fileInfo.Size() + int64(s.bufWriter.Buffered()), nil
}

// sync flush all entries in buffer to disk and enforce the OS to write immediately to file by using fsync()
func (s *segment) sync() error {
	if err := s.bufWriter.Flush(); err != nil {
		return err
	}

	return s.file.Sync()
}

// readSegment return all entries in segment file and latest checkpoint sequence number
func readSegment(path string) ([]Entry, uint64, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if errClose := fd.Close(); errClose != nil {
			fmt.Printf("Error closing file description when readSegment: %s\n", errClose)
		}
	}()

	entries := make([]Entry, 0)
	var checkpointSeq uint64

	for {
		var size uint32
		if err = binary.Read(fd, binary.LittleEndian, &size); err != nil {
			if err == io.EOF {
				break
			}
			return entries, checkpointSeq, err
		}

		body := make([]byte, size)
		if _, err = io.ReadFull(fd, body); err != nil {
			return entries, checkpointSeq, err
		}

		entry, errDecode := decodeEntry(body)
		if errDecode != nil {
			return entries, checkpointSeq, errDecode
		}

		entries = append(entries, entry)
		if entry.IsCheckpoint {
			checkpointSeq = entry.SequenceNumber
		}
	}

	return entries, checkpointSeq, nil
}
