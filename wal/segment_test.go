package wal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSegment(t *testing.T) {
	t.Run("create segment file", func(t *testing.T) {
		dir := t.TempDir()

		seg, err := createSegment(dir, 0)
		require.NoError(t, err)
		defer seg.close()

		// check the file name
		assert.Equal(t, "segment-0", seg.file.Name())
	})

	t.Run("append, flush, read from disk -> should have data", func(t *testing.T) {
		seg, err := createSegment(t.TempDir(), 0)
		require.NoError(t, err)
		defer seg.close()

		err = seg.append(Entry{
			SequenceNumber: 1,
			Data:           []byte("data"),
			IsCheckpoint:   true,
		})
		err = seg.append(Entry{
			SequenceNumber: 2,
			Data:           []byte("data"),
		})
		require.NoError(t, err)

		err = seg.sync()
		require.NoError(t, err)

		entries, checkpointSeq, err := seg.readAll()
		require.NoError(t, err)
		assert.Len(t, entries, 2)
		assert.Len(t, checkpointSeq, 1)
		assert.Equal(t, uint64(1), entries[0].SequenceNumber)
		assert.Equal(t, uint64(2), entries[1].SequenceNumber)
	})

	t.Run("append, no flush, read from disk -> no data", func(t *testing.T) {
		seg, err := createSegment(t.TempDir(), 0)
		require.NoError(t, err)
		defer seg.close()

		err = seg.append(Entry{
			SequenceNumber: 1,
			Data:           []byte("data"),
			IsCheckpoint:   true,
		})
		err = seg.append(Entry{
			SequenceNumber: 2,
			Data:           []byte("data"),
		})
		require.NoError(t, err)

		entries, _, err := seg.readAll()
		require.NoError(t, err)
		assert.Len(t, entries, 0)
	})

	t.Run("function size reflected the flushed and buffered bytes", func(t *testing.T) {
		seg, err := createSegment(t.TempDir(), 0)
		require.NoError(t, err)
		defer seg.close()

		err = seg.append(Entry{
			SequenceNumber: 1,
			Data:           []byte("data"),
			IsCheckpoint:   true,
		})
		err = seg.append(Entry{
			SequenceNumber: 2,
			Data:           []byte("data"),
		})
		require.NoError(t, err)

		err = seg.sync()
		require.NoError(t, err)

		err = seg.append(Entry{
			SequenceNumber: 3,
			Data:           []byte("data"),
		})

		segSize, err := seg.size()
		require.NoError(t, err)
		assert.Equal(t, uint64(3), segSize)
	})
}
