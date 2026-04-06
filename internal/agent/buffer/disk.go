package buffer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// DiskBuffer manages a series of append-only WAL segments for reliable buffering.
type DiskBuffer struct {
	mu             sync.Mutex
	dir            string
	maxSegmentSize int64
	maxTotalSize   int64
	active         *Segment
	seqNum         int
	dataLoss       atomic.Int64
}

// NewDiskBuffer creates or opens a disk-backed buffer in the given directory.
func NewDiskBuffer(dir string, maxSegmentSize, maxTotalSize int64) (*DiskBuffer, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create buffer dir: %w", err)
	}

	db := &DiskBuffer{
		dir:            dir,
		maxSegmentSize: maxSegmentSize,
		maxTotalSize:   maxTotalSize,
	}

	// Find the highest existing sequence number.
	segments, _ := db.listSegments()
	if len(segments) > 0 {
		last := segments[len(segments)-1]
		name := filepath.Base(last)
		fmt.Sscanf(name, "segment-%06d.wal", &db.seqNum)
	}

	// Create a new active segment.
	if err := db.rotateSegment(); err != nil {
		return nil, err
	}

	return db, nil
}

// Write appends data to the active segment, rotating if needed.
func (db *DiskBuffer) Write(data []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Enforce total size limit.
	totalSize := db.totalSizeLocked()
	if totalSize+int64(len(data)) > db.maxTotalSize {
		if err := db.evictOldestLocked(); err != nil {
			db.dataLoss.Add(1)
			return nil // drop silently, increment data loss counter
		}
	}

	// Rotate if active segment is full.
	if db.active.Size() >= db.maxSegmentSize {
		if err := db.rotateSegmentLocked(); err != nil {
			return err
		}
	}

	return db.active.Append(data)
}

// ReadAll reads all entries from the oldest unread segment.
// Returns entries and the segment path for acknowledgment.
func (db *DiskBuffer) ReadAll() ([][]byte, string, error) {
	db.mu.Lock()
	segments, err := db.listSegments()
	db.mu.Unlock()
	if err != nil || len(segments) == 0 {
		return nil, "", err
	}

	// Read from the oldest segment (first in sorted order).
	oldest := segments[0]

	// Skip the active segment.
	db.mu.Lock()
	activePath := db.active.Path()
	db.mu.Unlock()
	if oldest == activePath {
		return nil, "", nil
	}

	reader, err := NewSegmentReader(oldest)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var entries [][]byte
	for {
		entry, err := reader.ReadNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Corrupted entry — skip and continue.
			break
		}
		entries = append(entries, entry)
	}

	return entries, oldest, nil
}

// Ack removes a fully-shipped segment file.
func (db *DiskBuffer) Ack(segmentPath string) error {
	return os.Remove(segmentPath)
}

// DataLossCount returns the number of entries dropped due to buffer overflow.
func (db *DiskBuffer) DataLossCount() int64 {
	return db.dataLoss.Load()
}

// TotalSize returns the total disk usage of all segments.
func (db *DiskBuffer) TotalSize() int64 {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.totalSizeLocked()
}

// SegmentCount returns the number of segment files.
func (db *DiskBuffer) SegmentCount() int {
	db.mu.Lock()
	defer db.mu.Unlock()
	segments, _ := db.listSegments()
	return len(segments)
}

// Close syncs and closes the active segment.
func (db *DiskBuffer) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.active != nil {
		db.active.Sync()
		return db.active.Close()
	}
	return nil
}

func (db *DiskBuffer) rotateSegment() error {
	return db.rotateSegmentLocked()
}

func (db *DiskBuffer) rotateSegmentLocked() error {
	if db.active != nil {
		db.active.Sync()
		db.active.Close()
	}

	db.seqNum++
	path := filepath.Join(db.dir, fmt.Sprintf("segment-%06d.wal", db.seqNum))
	seg, err := CreateSegment(path)
	if err != nil {
		return err
	}
	db.active = seg
	return nil
}

func (db *DiskBuffer) listSegments() ([]string, error) {
	entries, err := os.ReadDir(db.dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".wal") {
			paths = append(paths, filepath.Join(db.dir, e.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func (db *DiskBuffer) totalSizeLocked() int64 {
	segments, _ := db.listSegments()
	var total int64
	for _, p := range segments {
		if info, err := os.Stat(p); err == nil {
			total += info.Size()
		}
	}
	return total
}

func (db *DiskBuffer) evictOldestLocked() error {
	segments, err := db.listSegments()
	if err != nil || len(segments) <= 1 {
		return fmt.Errorf("no segments to evict")
	}
	// Never evict the active segment.
	oldest := segments[0]
	if oldest == db.active.Path() {
		return fmt.Errorf("cannot evict active segment")
	}
	return os.Remove(oldest)
}
