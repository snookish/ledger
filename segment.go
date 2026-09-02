package ledger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// segmentName builds the file name for a segment number.
func segmentName(segmentNumber int) string {
	return fmt.Sprintf("%08d.log", segmentNumber)
}

// segmentPath joins dir and segment name.
func segmentPath(dir string, segmentNumber int) string {
	return filepath.Join(dir, segmentName(segmentNumber))
}

// parseSegmentName extracts the number from a file name like 00000001.log.
func parseSegmentName(name string) (int, bool) {
	if !strings.HasSuffix(name, ".log") {
		return 0, false
	}

	base := strings.TrimSuffix(name, ".log")
	if len(base) != 8 {
		return 0, false
	}

	segmentNumber, err := strconv.Atoi(base)
	if err != nil {
		return 0, false
	}

	return segmentNumber, true
}

// listSegments returns all segment paths in dir, sorted oldest first.
func listSegments(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var segments []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if _, ok := parseSegmentName(entry.Name()); !ok {
			continue
		}

		segments = append(segments, filepath.Join(dir, entry.Name()))
	}

	sort.Strings(segments)
	return segments, nil
}

// nextSegmentNum finds the next number to use.
func nextSegmentNum(dir string) (int, error) {
	segments, err := listSegments(dir)
	if err != nil {
		return 0, err
	}

	if len(segments) == 0 {
		return 1, nil
	}

	lastSegment := filepath.Base(segments[len(segments)-1])
	segmentNumber, ok := parseSegmentName(lastSegment)
	if !ok {
		return 0, fmt.Errorf("bad segment name %q", lastSegment)
	}

	return segmentNumber + 1, nil
}

// fsyncDir flushes the directory entry so a new file survives a crash.
func fsyncDir(dir string) error {
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = dirFile.Close() }()

	return syncFile(dirFile)
}

// syncFile can be swapped in tests to force a sync failure.
var syncFile = func(file *os.File) error {
	return file.Sync()
}

// openSegmentForAppend opens a segment for appending and fsyncs the dir.
// It creates the file if needed and always writes at the end.
func openSegmentForAppend(dir string, segmentNumber int) (*os.File, error) {
	path := segmentPath(dir, segmentNumber)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	if err := fsyncDir(dir); err != nil {
		_ = file.Close()
		return nil, err
	}

	return file, nil
}
