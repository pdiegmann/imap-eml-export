package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// IMAPClient defines the interface the exporter uses.
type IMAPClient interface {
	ListFolders() ([]string, error)
	FetchMessages(folder string) ([]Message, error)
}

// Message represents a single email message.
type Message struct {
	UID     uint32
	Subject string
	Date    time.Time
	Raw     []byte
}

// ProgressUpdate carries progress information for display.
type ProgressUpdate struct {
	Folder           string
	Current          int
	Total            int
	BytesTransferred int64
}

// ProgressCallback is called with progress updates during export.
type ProgressCallback func(update ProgressUpdate)

// Exporter handles the IMAP-to-EML export process.
type Exporter struct {
	OutputDir string
	Progress  ProgressCallback
}

// New creates a new Exporter.
func New(outputDir string, progress ProgressCallback) *Exporter {
	return &Exporter{
		OutputDir: outputDir,
		Progress:  progress,
	}
}

// Export fetches all messages from all folders and writes them as .eml files.
func (e *Exporter) Export(ctx context.Context, client IMAPClient) error {
	folders, err := client.ListFolders()
	if err != nil {
		return fmt.Errorf("listing folders: %w", err)
	}

	for _, folder := range folders {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := e.exportFolder(ctx, client, folder); err != nil {
			return fmt.Errorf("exporting folder %q: %w", folder, err)
		}
	}
	return nil
}

func (e *Exporter) exportFolder(ctx context.Context, client IMAPClient, folder string) error {
	messages, err := client.FetchMessages(folder)
	if err != nil {
		return err
	}

	folderDir := folderToPath(e.OutputDir, folder)
	if err := os.MkdirAll(folderDir, 0o755); err != nil {
		return fmt.Errorf("creating folder dir: %w", err)
	}

	total := len(messages)
	var bytesDownloaded int64

	for i, msg := range messages {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		filename := GenerateFilename(i+1, msg.Date, msg.Subject)
		filename = DeduplicateFilename(folderDir, filename)
		fullPath := filepath.Join(folderDir, filename)

		if err := os.WriteFile(fullPath, msg.Raw, 0o644); err != nil {
			return fmt.Errorf("writing message %d: %w", msg.UID, err)
		}
		bytesDownloaded += int64(len(msg.Raw))

		if e.Progress != nil {
			e.Progress(ProgressUpdate{
				Folder:           folder,
				Current:          i + 1,
				Total:            total,
				BytesTransferred: bytesDownloaded,
			})
		}
	}
	return nil
}

// folderToPath maps an IMAP folder name to a filesystem path.
func folderToPath(base, folder string) string {
	parts := strings.Split(folder, ".")
	return filepath.Join(append([]string{base}, parts...)...)
}
