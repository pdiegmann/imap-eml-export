package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pdiegmann/imap-eml-export/internal/export"
)

// IMAPImportClient defines the target-IMAP operations the importer needs.
type IMAPImportClient interface {
	// EnsureMailbox creates the mailbox if it does not already exist.
	EnsureMailbox(name string) error
	// AppendMessage uploads raw EML bytes to the named mailbox.
	AppendMessage(folder string, date time.Time, raw []byte) error
}

// ProgressCallback is called with progress updates during import.
// It deliberately reuses export.ProgressUpdate so the same TUI widget can be
// driven for both directions.
type ProgressCallback func(update export.ProgressUpdate)

// Importer reads EML files from a local directory tree and uploads them to an
// IMAP server, recreating the original folder hierarchy.
type Importer struct {
	InputDir string
	Progress ProgressCallback
}

// New creates a new Importer for the given source directory.
func New(inputDir string, progress ProgressCallback) *Importer {
	return &Importer{
		InputDir: inputDir,
		Progress: progress,
	}
}

// Import walks InputDir, ensures every mailbox exists on the target server,
// and appends all discovered .eml files to their corresponding mailbox.
func (im *Importer) Import(ctx context.Context, client IMAPImportClient) error {
	folders, err := im.collectFolders()
	if err != nil {
		return fmt.Errorf("scanning input directory: %w", err)
	}

	for _, f := range folders {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := im.importFolder(ctx, client, f); err != nil {
			return fmt.Errorf("importing folder %q: %w", f.mailbox, err)
		}
	}
	return nil
}

// folderEntry pairs an IMAP mailbox name with its local directory.
type folderEntry struct {
	mailbox string // e.g. "Work.ProjectA"
	dir     string // absolute path on disk
}

// collectFolders returns one entry per subdirectory that holds at least one
// .eml file.  The root of InputDir maps to "INBOX" only when it directly
// contains .eml files; otherwise it is skipped.
func (im *Importer) collectFolders() ([]folderEntry, error) {
	var result []folderEntry

	err := filepath.WalkDir(im.InputDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(im.InputDir, path)
		if err != nil {
			return err
		}

		var mailboxName string
		if rel == "." {
			// Check whether the root directory itself contains .eml files.
			has, err := dirHasEML(path)
			if err != nil {
				return err
			}
			if !has {
				return nil
			}
			mailboxName = "INBOX"
		} else {
			has, err := dirHasEML(path)
			if err != nil {
				return err
			}
			if !has {
				return nil
			}
			mailboxName = pathToMailbox(rel)
		}

		result = append(result, folderEntry{mailbox: mailboxName, dir: path})
		return nil
	})
	return result, err
}

// dirHasEML returns true when dir contains at least one file ending in .eml.
func dirHasEML(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".eml") {
			return true, nil
		}
	}
	return false, nil
}

// pathToMailbox converts a relative filesystem path to an IMAP mailbox name.
// Example: "Work/ProjectA" → "Work.ProjectA"
func pathToMailbox(relPath string) string {
	// filepath.ToSlash normalises Windows backslashes.
	return strings.ReplaceAll(filepath.ToSlash(relPath), "/", ".")
}

func (im *Importer) importFolder(ctx context.Context, client IMAPImportClient, folder folderEntry) error {
	entries, err := os.ReadDir(folder.dir)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	var emlFiles []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".eml") {
			emlFiles = append(emlFiles, e)
		}
	}
	if len(emlFiles) == 0 {
		return nil
	}

	if err := client.EnsureMailbox(folder.mailbox); err != nil {
		return fmt.Errorf("ensuring mailbox: %w", err)
	}

	total := len(emlFiles)
	var bytesUploaded int64

	for i, entry := range emlFiles {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fullPath := filepath.Join(folder.dir, entry.Name())
		raw, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("reading %q: %w", fullPath, err)
		}

		// Use file mtime as the IMAP internal date when available.
		var msgDate time.Time
		if info, err := entry.Info(); err == nil {
			msgDate = info.ModTime()
		}

		if err := client.AppendMessage(folder.mailbox, msgDate, raw); err != nil {
			return fmt.Errorf("appending %q: %w", entry.Name(), err)
		}

		bytesUploaded += int64(len(raw))

		if im.Progress != nil {
			im.Progress(export.ProgressUpdate{
				Folder:           folder.mailbox,
				Current:          i + 1,
				Total:            total,
				BytesTransferred: bytesUploaded,
			})
		}
	}
	return nil
}
