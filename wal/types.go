package wal

// Entry represents for one record in WAL
type Entry struct {
	SequenceNumber uint64
	Data           []byte
	CRC            uint32 // checksum
	IsCheckpoint   bool   // used to mark this entry is checkpointed or not
}

type Options struct {
	MaxFileSize int64
	MaxSegments int
}

var DefaultOptions = Options{
	MaxFileSize: 64 * 1024 * 1024,
	MaxSegments: 1000,
}
