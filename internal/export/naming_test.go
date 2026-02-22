package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Test/File\\Name", "test-file-name"},
		{"  leading-trailing  ", "leading-trailing"},
		{"", "no-subject"},
		{"UPPER CASE", "upper-case"},
		{"file@#$%name", "filename"},
		{strings.Repeat("a", 60), strings.Repeat("a", 50)},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeFilename(tt.input))
		})
	}
}

func TestGenerateFilename(t *testing.T) {
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := GenerateFilename(1, date, "Hello World")
	assert.Equal(t, "00001_2024-01-15_hello-world.eml", result)

	result = GenerateFilename(99999, date, "Test Subject")
	assert.Equal(t, "99999_2024-01-15_test-subject.eml", result)
}

func TestIsWindowsReserved(t *testing.T) {
	assert.True(t, IsWindowsReserved("CON"))
	assert.True(t, IsWindowsReserved("NUL"))
	assert.True(t, IsWindowsReserved("PRN"))
	assert.True(t, IsWindowsReserved("COM1"))
	assert.True(t, IsWindowsReserved("LPT9"))
	assert.False(t, IsWindowsReserved("config"))
	assert.False(t, IsWindowsReserved("normal.eml"))
}

func TestDeduplicateFilename(t *testing.T) {
	dir := t.TempDir()

	result := DeduplicateFilename(dir, "test.eml")
	assert.Equal(t, "test.eml", result)

	os.WriteFile(filepath.Join(dir, "test.eml"), []byte{}, 0o600)

	result = DeduplicateFilename(dir, "test.eml")
	assert.Equal(t, "test_1.eml", result)

	os.WriteFile(filepath.Join(dir, "test_1.eml"), []byte{}, 0o600)

	result = DeduplicateFilename(dir, "test.eml")
	assert.Equal(t, "test_2.eml", result)
}
