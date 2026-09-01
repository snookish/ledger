package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Extra errors for validation.
var ErrBadDir = errors.New("ledger: bad dir")

// validateDir checks the dir before we try to use it.
func validateDir(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return ErrBadDir
	}

	if strings.Contains(dir, "\x00") {
		return ErrBadDir
	}

	cleaned := filepath.Clean(dir)
	if cleaned == "." {
		return ErrBadDir
	}

	if !filepath.IsAbs(cleaned) {
		return ErrBadDir
	}

	if info, err := os.Stat(cleaned); err == nil && !info.IsDir() {
		return ErrBadDir
	}

	return nil
}
