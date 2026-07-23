package wal

import (
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
