package export

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var (
	unsafeCharsRe     = regexp.MustCompile(`[^a-z0-9\-_]`)
	windowsReservedRe = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])$`)
)

// SanitizeFilename removes unsafe characters and truncates to 50 chars.
func SanitizeFilename(subject string) string {
	s := strings.ToLower(subject)
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '/' || r == '\\' {
			return '-'
		}
		return r
	}, s)
	s = unsafeCharsRe.ReplaceAllString(s, "")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-_")
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-_")
	}
	if s == "" {
		s = "no-subject"
	}
	return s
}

// GenerateFilename produces a filename like "00001_2024-01-15_subject.eml".
func GenerateFilename(seq int, date time.Time, subject string) string {
	sanitized := SanitizeFilename(subject)
	dateStr := date.Format("2006-01-02")
	return fmt.Sprintf("%05d_%s_%s.eml", seq, dateStr, sanitized)
}

// IsWindowsReserved returns true if the name is a Windows reserved filename.
func IsWindowsReserved(name string) bool {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return windowsReservedRe.MatchString(base)
}

// DeduplicateFilename returns a unique filename in dir by appending _N if needed.
func DeduplicateFilename(dir, base string) string {
	candidate := base
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	for i := 1; ; i++ {
		full := filepath.Join(dir, candidate)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d%s", stem, i, ext)
	}
}
