package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
		goWal, err := NewWAL(dir, DefaultOptions)
		require.NoError(t, err)

		defer func() {
			require.NoError(t, goWal.Close())
		}()
		path := filepath.Join(dir, fmt.Sprintf("%s%d.wal", SEGMENT_PREFIX, 0))
		_, err = os.Stat(path)
		require.NoError(t, err)
	})

	t.Run("recover latest LSN when last segment file is empty", func(t *testing.T) {
		dir := t.TempDir()

		w, err := NewWAL(dir, DefaultOptions)
		require.NoError(t, err)
		require.NoError(t, w.Write([]byte("a")))
		require.NoError(t, w.Write([]byte("b")))
		require.NoError(t, w.Sync())
		require.NoError(t, w.Close())

		// mock the rotation for segment file
		emptySeg, err := os.Create(segmentPath(dir, 1))
		require.NoError(t, err)
		require.NoError(t, emptySeg.Close())

		w2, err := NewWAL(dir, DefaultOptions)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, w2.Close())
		}()

		require.NoError(t, w2.Write([]byte("c")))
		require.NoError(t, w2.Sync())

		entries, err := w2.ReadAll(false)
		require.NoError(t, err)
		assert.Equal(t, uint64(3), entries[0].SequenceNumber)
	})

	t.Run("recover LSN in normal case", func(t *testing.T) {
		dir := t.TempDir()
		w, err := NewWAL(dir, DefaultOptions)
		require.NoError(t, err)
		require.NoError(t, w.Write([]byte("a")))
		require.NoError(t, w.Sync())
		require.NoError(t, w.Close())

		w2, err := NewWAL(dir, DefaultOptions)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, w2.Close())
		}()

		require.NoError(t, w2.Write([]byte("b")))
		require.NoError(t, w2.Sync())

		entries, err := w2.ReadAll(false)
		require.NoError(t, err)
		assert.Len(t, entries, 2)
		assert.Equal(t, uint64(2), entries[1].SequenceNumber)
	})

	t.Run("concurrent write -> must thread safe about the lsn", func(t *testing.T) {
		dir := t.TempDir()
		w, err := NewWAL(dir, DefaultOptions)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, w.Close())
		}()
		wg := sync.WaitGroup{}

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				require.NoError(t, w.Write([]byte(fmt.Sprintf("data-%d", i))))
			}()
		}
		wg.Wait()
		require.NoError(t, w.Sync())

		entries, err := w.ReadAll(false)
		require.NoError(t, err)
		assert.Len(t, entries, 10)
		for idx, entry := range entries {
			assert.Equal(t, entry.SequenceNumber, uint64(idx+1))
		}
	})

	t.Run("create new segment when reach to the end size of segment", func(t *testing.T) {
		dir := t.TempDir()
		maxFileSizeBytes := int64(100)
		currentSegment, err := createSegment(dir, 0)
		require.NoError(t, err)
		w := &wal{dir: dir, opts: Options{
			MaxFileSize: maxFileSizeBytes,
			MaxSegments: 1000,
		}, currentSegment: currentSegment}
		defer func() {
			require.NoError(t, w.Close())
		}()

		for {
			require.NoError(t, w.Write([]byte("data")))
			entries, errDir := os.ReadDir(dir)
			require.NoError(t, errDir)

			currentSize, errSize := w.currentSegment.size()
			require.NoError(t, errSize)
			if currentSize > maxFileSizeBytes {
				t.Fatal("segment is too large but still not create new segment")
			}

			if len(entries) == 2 {
				assert.Equal(t, "segment-0.wal", entries[0].Name())
				assert.Equal(t, "segment-1.wal", entries[1].Name())
				break
			}
		}
	})

	t.Run("skip record before checkpoint", func(t *testing.T) {
		dir := t.TempDir()
		w, err := NewWAL(dir, DefaultOptions)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, w.Close())
		}()

		require.NoError(t, w.Write([]byte("old-1")))
		require.NoError(t, w.Write([]byte("old-2")))
		require.NoError(t, w.CreateCheckpoint([]byte("snapshot")))
		require.NoError(t, w.Write([]byte("new-1")))
		require.NoError(t, w.Write([]byte("new-2")))
		require.NoError(t, w.Write([]byte("new-3")))
		require.NoError(t, w.Sync())

		entries, err := w.ReadAll(true)
		require.NoError(t, err)

		assert.Len(t, entries, 4)
		assert.Equal(t, uint64(3), entries[0].SequenceNumber)
		assert.Equal(t, uint64(4), entries[1].SequenceNumber)
		assert.Equal(t, uint64(5), entries[2].SequenceNumber)
		assert.Equal(t, uint64(6), entries[3].SequenceNumber)
	})

	t.Run("delete oldest segment when max segment is reached", func(t *testing.T) {
		dir := t.TempDir()
		maxFileSizeBytes := int64(100)
		currentSegment, err := createSegment(dir, 0)
		require.NoError(t, err)
		w := &wal{dir: dir, opts: Options{
			MaxFileSize: maxFileSizeBytes,
			MaxSegments: 1,
		}, currentSegment: currentSegment}
		defer func() {
			require.NoError(t, w.Close())
		}()

		for {
			require.NoError(t, w.Write([]byte("data")))

			if w.currentSegment.index != currentSegment.index {
				break
			}
		}

		entries, errDir := os.ReadDir(dir)
		require.NoError(t, errDir)
		require.Len(t, entries, 1)
		assert.Equal(t, "segment-1.wal", entries[0].Name())
	})
}
