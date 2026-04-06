package buffer

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

const headerSize = 4 // uint32 length prefix
const checksumSize = 4 // CRC32

// Segment is an append-only file for buffering serialized batches.
// Format per entry: [4-byte length][payload][4-byte CRC32]
type Segment struct {
	mu   sync.Mutex
	f    *os.File
	path string
	size int64
}

// CreateSegment creates a new segment file for writing.
func CreateSegment(path string) (*Segment, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create segment %s: %w", path, err)
	}
	return &Segment{f: f, path: path, size: 0}, nil
}

// OpenSegment opens an existing segment file and seeks to the end for appending.
func OpenSegment(path string) (*Segment, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open segment %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat segment %s: %w", path, err)
	}
	return &Segment{f: f, path: path, size: info.Size()}, nil
}

// Append writes a single entry to the segment.
func (s *Segment) Append(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	length := uint32(len(data))
	checksum := crc32.ChecksumIEEE(data)

	// Write length prefix.
	if err := binary.Write(s.f, binary.BigEndian, length); err != nil {
		return fmt.Errorf("write length: %w", err)
	}
	// Write payload.
	if _, err := s.f.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	// Write checksum.
	if err := binary.Write(s.f, binary.BigEndian, checksum); err != nil {
		return fmt.Errorf("write checksum: %w", err)
	}

	s.size += int64(headerSize + len(data) + checksumSize)
	return nil
}

// Size returns the current segment file size in bytes.
func (s *Segment) Size() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.size
}

// Path returns the file path of the segment.
func (s *Segment) Path() string {
	return s.path
}

// Sync flushes the segment to disk.
func (s *Segment) Sync() error {
	return s.f.Sync()
}

// Close closes the segment file.
func (s *Segment) Close() error {
	return s.f.Close()
}

// SegmentReader reads entries sequentially from a segment file.
type SegmentReader struct {
	f *os.File
}

// NewSegmentReader opens a segment for sequential reading.
func NewSegmentReader(path string) (*SegmentReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open segment for reading %s: %w", path, err)
	}
	return &SegmentReader{f: f}, nil
}

// ReadNext reads the next entry. Returns io.EOF when no more entries.
func (r *SegmentReader) ReadNext() ([]byte, error) {
	var length uint32
	if err := binary.Read(r.f, binary.BigEndian, &length); err != nil {
		return nil, err // io.EOF is valid here
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r.f, data); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	var storedChecksum uint32
	if err := binary.Read(r.f, binary.BigEndian, &storedChecksum); err != nil {
		return nil, fmt.Errorf("read checksum: %w", err)
	}

	computed := crc32.ChecksumIEEE(data)
	if computed != storedChecksum {
		return nil, fmt.Errorf("checksum mismatch: stored=%x computed=%x", storedChecksum, computed)
	}

	return data, nil
}

// Close closes the reader.
func (r *SegmentReader) Close() error {
	return r.f.Close()
}
