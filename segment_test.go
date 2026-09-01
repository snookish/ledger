package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSegmentName(t *testing.T) {
	if n, ok := parseSegmentName("00000001.log"); !ok || n != 1 {
		t.Fatalf("want 1, got %d ok %v", n, ok)
	}
	if _, ok := parseSegmentName("00000001.txt"); ok {
		t.Fatal("should reject bad suffix")
	}
	if _, ok := parseSegmentName("1.log"); ok {
		t.Fatal("should reject short name")
	}
	if _, ok := parseSegmentName("abcdefgh.log"); ok {
		t.Fatal("should reject non numeric")
	}
}

func TestListSegmentsSorted(t *testing.T) {
	dir := t.TempDir()
	// Noise that should be ignored.
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)
	os.Mkdir(filepath.Join(dir, "00000099.log"), 0o755)
	for _, n := range []int{3, 1, 2} {
		f, _ := os.Create(segmentPath(dir, n))
		f.Close()
	}
	segs, err := listSegments(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 3 {
		t.Fatalf("want 3, got %d", len(segs))
	}
	if filepath.Base(segs[0]) != "00000001.log" || filepath.Base(segs[2]) != "00000003.log" {
		t.Fatal("not sorted")
	}
}

func TestNextSegmentNum(t *testing.T) {
	dir := t.TempDir()
	if n, _ := nextSegmentNum(dir); n != 1 {
		t.Fatalf("empty dir should start at 1, got %d", n)
	}
	for _, n := range []int{1, 5} {
		f, _ := os.Create(segmentPath(dir, n))
		f.Close()
	}
	if n, _ := nextSegmentNum(dir); n != 6 {
		t.Fatalf("want 6, got %d", n)
	}
}

func TestOpenSegmentForAppend(t *testing.T) {
	dir := t.TempDir()
	f, err := openSegmentForAppend(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("hello"))
	f.Close()

	// Reopen same segment, should append not truncate.
	f2, err := openSegmentForAppend(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	f2.Write([]byte(" world"))
	f2.Close()

	data, _ := os.ReadFile(segmentPath(dir, 1))
	if string(data) != "hello world" {
		t.Fatalf("want hello world, got %q", string(data))
	}
}

func TestListSegmentsError(t *testing.T) {
	if _, err := listSegments("/no/such/dir/xyz"); err == nil {
		t.Fatal("want error for missing dir")
	}
}

func TestNextSegmentNumError(t *testing.T) {
	if _, err := nextSegmentNum("/no/such/dir/xyz"); err == nil {
		t.Fatal("want error for missing dir")
	}
}

func TestFsyncDirError(t *testing.T) {
	if err := fsyncDir("/no/such/dir/xyz"); err == nil {
		t.Fatal("want error for missing dir")
	}
}

func TestFsyncDirSyncError(t *testing.T) {
	original := syncFile
	syncFile = func(*os.File) error { return os.ErrInvalid }
	defer func() { syncFile = original }()

	dir := t.TempDir()
	if err := fsyncDir(dir); err == nil {
		t.Fatal("want error from sync")
	}
}

func TestOpenSegmentForAppendError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file")
	os.WriteFile(filePath, []byte("x"), 0o644)
	if _, err := openSegmentForAppend(filePath, 1); err == nil {
		t.Fatal("want error when dir is file")
	}
}

func TestOpenSegmentForAppendFsyncError(t *testing.T) {
	original := syncFile
	syncFile = func(*os.File) error { return os.ErrInvalid }
	defer func() { syncFile = original }()

	dir := t.TempDir()
	if _, err := openSegmentForAppend(dir, 1); err == nil {
		t.Fatal("want error from fsync")
	}
}
