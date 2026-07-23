package wal

import (
	"bufio"
	"os"
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

func createSegment(dir string, index int) (*segment, error) {
	panic("not implement")
}

func (s *segment) append(e Entry) error {
	panic("not implement")
}

func (s *segment) close() error {
	panic("not implement")
}

func (s *segment) size() (int64, error) {
	panic("not implement")
}

func (s *segment) sync() error {
	panic("not implement")
}

func (s *segment) readAll() ([]Entry, uint64, error) {
	panic("not implement")
}
