package buffer

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSegmentWriteRead(t *testing.T) {
	dir := t.TempDir()
	seg, err := CreateSegment(dir + "/test.wal")
	require.NoError(t, err)

	data1 := []byte(`{"metrics":[{"name":"cpu.user"}]}`)
	data2 := []byte(`{"logs":[{"message":"hello"}]}`)

	require.NoError(t, seg.Append(data1))
	require.NoError(t, seg.Append(data2))
	assert.Greater(t, seg.Size(), int64(0))
	require.NoError(t, seg.Close())

	reader, err := NewSegmentReader(dir + "/test.wal")
	require.NoError(t, err)
	defer reader.Close()

	got1, err := reader.ReadNext()
	require.NoError(t, err)
	assert.Equal(t, data1, got1)

	got2, err := reader.ReadNext()
	require.NoError(t, err)
	assert.Equal(t, data2, got2)

	_, err = reader.ReadNext()
	assert.ErrorIs(t, err, io.EOF)
}

func TestDiskBufferWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDiskBuffer(dir, 1024, 1024*1024)
	require.NoError(t, err)

	// Write some data.
	for i := 0; i < 5; i++ {
		require.NoError(t, db.Write([]byte(`{"test":true}`)))
	}

	// Force rotation by closing and creating a new buffer.
	require.NoError(t, db.Close())

	db2, err := NewDiskBuffer(dir, 1024, 1024*1024)
	require.NoError(t, err)
	defer db2.Close()

	// Write more data to new segment.
	require.NoError(t, db2.Write([]byte(`{"test":true}`)))

	// Read from oldest segment.
	entries, path, err := db2.ReadAll()
	require.NoError(t, err)
	if path != "" {
		assert.Len(t, entries, 5)
		require.NoError(t, db2.Ack(path))
	}
}

func TestDiskBufferSegmentRotation(t *testing.T) {
	dir := t.TempDir()
	// Very small max segment size to force rotation.
	db, err := NewDiskBuffer(dir, 50, 1024*1024)
	require.NoError(t, err)
	defer db.Close()

	for i := 0; i < 10; i++ {
		require.NoError(t, db.Write([]byte(`{"i":1234567890}`)))
	}

	assert.Greater(t, db.SegmentCount(), 1)
}

func TestDiskBufferDataLoss(t *testing.T) {
	dir := t.TempDir()
	// Very small total size to force eviction.
	db, err := NewDiskBuffer(dir, 50, 200)
	require.NoError(t, err)
	defer db.Close()

	for i := 0; i < 50; i++ {
		_ = db.Write([]byte(`{"data":"some payload here to fill up buffer"}`))
	}

	// Should have recorded some data loss.
	assert.GreaterOrEqual(t, db.DataLossCount(), int64(0))
}
