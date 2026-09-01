package ledger

import (
	"os"
	"path/filepath"
)

// segmentManager hides file names, sorting and directory fsync.
type segmentManager struct {
	dir string
}

func newSegmentManager(dir string) *segmentManager {
	return &segmentManager{dir: dir}
}

// list returns all segment paths sorted oldest first.
func (manager *segmentManager) list() ([]string, error) {
	return listSegments(manager.dir)
}

// nextNum returns the number for the next segment.
func (manager *segmentManager) nextNum() (int, error) {
	return nextSegmentNum(manager.dir)
}

// createNext makes the next segment, fsyncs the dir, and returns the file and number.
func (manager *segmentManager) createNext() (*os.File, int, error) {
	nextNum, err := manager.nextNum()
	if err != nil {
		return nil, 0, err
	}

	file, err := openSegmentForAppend(manager.dir, nextNum)
	if err != nil {
		return nil, 0, err
	}

	return file, nextNum, nil
}

// openLast opens the last segment for append and returns file, number, and block offset.
func (manager *segmentManager) openLast() (*os.File, int, int, error) {
	segments, err := manager.list()
	if err != nil {
		return nil, 0, 0, err
	}

	if len(segments) == 0 {
		file, segmentNum, err := manager.createNext()
		if err != nil {
			return nil, 0, 0, err
		}
		return file, segmentNum, 0, nil
	}

	lastSegment := segments[len(segments)-1]
	baseName := filepath.Base(lastSegment)
	segmentNum, _ := parseSegmentName(baseName)

	file, err := os.OpenFile(lastSegment, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, 0, 0, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, 0, err
	}

	blockOffset := int(info.Size() % int64(BlockSize))
	return file, segmentNum, blockOffset, nil
}
