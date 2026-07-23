package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeEntry(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		e := Entry{
			SequenceNumber: 1,
			Data:           []byte("hello world"),
			IsCheckpoint:   true,
		}

		encoded, err := encodeEntry(e)
		require.NoError(t, err)

		decoded, err := decodeEntry(encoded)

		require.NoError(t, err)
		assert.Equal(t, e.SequenceNumber, decoded.SequenceNumber)
		assert.True(t, decoded.IsCheckpoint)
		assert.Equal(t, e.Data, decoded.Data)
	})

	t.Run("corrupted data", func(t *testing.T) {
		e := Entry{
			SequenceNumber: 1,
			Data:           []byte("hello world"),
			IsCheckpoint:   true,
		}

		encoded, err := encodeEntry(e)
		require.NoError(t, err)
		encoded[len(encoded)-1] ^= 0xFF

		_, err = decodeEntry(encoded)
		assert.Equal(t, ErrCRCMismatch, err)
	})

	// this test to ensure the function encode will not mutate the input data
	// the encode function should copy data then mutate it after.
	t.Run("don't mutate input data", func(t *testing.T) {
		original := make([]byte, 5, 20)
		copy(original, []byte("hello"))
		snapshot := append([]byte{}, original...)

		_, err := encodeEntry(Entry{SequenceNumber: 1, Data: original})

		require.NoError(t, err)
		assert.Equal(t, snapshot, original)
	})

	t.Run("truncated input fails cleanly", func(t *testing.T) {
		e := Entry{SequenceNumber: 1, Data: []byte("hello world")}
		encoded, err := encodeEntry(e)
		require.NoError(t, err)

		truncated := encoded[:len(encoded)-3]
		_, err = decodeEntry(truncated)
		assert.Equal(t, ErrTruncatedEntry, err)
	})
}

func TestWAL(t *testing.T) {
	// new wal -> directory -> created, segment if not exist -> create
	t.Run("create first segment when NewWAL in empty directory", func(t *testing.T) {
		dir := t.TempDir()
		goWal, err := NewWAL(dir, Options{})
		require.NoError(t, err)

		defer func() {
			require.NoError(t, goWal.Close())
		}()
		path := filepath.Join(dir, fmt.Sprintf("%s%d.wal", SEGMENT_PREFIX, 0))
		_, err = os.Stat(path)
		require.NoError(t, err)
	})

	// if write concurrent -> no co hoat dong ko

	// if write -> cai lsn (log sequence number) co hoat dong dung ko hay bi race condition

	// recover -> tim dc thang checkpoint moi nhat cua lsn, de replay events

	// to be able to create checkpoint by inserting new record in WAL

	// case test read all should skips all entries before checkpoint
}
