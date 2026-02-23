package importer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pdiegmann/imap-eml-export/internal/export"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- mock client ----

type mockImportClient struct {
	ensuredMailboxes  []string
	appendedMessages  []appendedMsg
	ensureErr         error
	appendErr         error
}

type appendedMsg struct {
	folder string
	date   time.Time
	raw    []byte
}

func (m *mockImportClient) EnsureMailbox(name string) error {
	if m.ensureErr != nil {
		return m.ensureErr
	}
	m.ensuredMailboxes = append(m.ensuredMailboxes, name)
	return nil
}

func (m *mockImportClient) AppendMessage(folder string, date time.Time, raw []byte) error {
	if m.appendErr != nil {
		return m.appendErr
	}
	m.appendedMessages = append(m.appendedMessages, appendedMsg{folder: folder, date: date, raw: raw})
	return nil
}

// ---- helpers ----

func writeEML(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

// ---- pathToMailbox tests ----

func TestPathToMailbox(t *testing.T) {
	tests := []struct {
		rel      string
		expected string
	}{
		{"INBOX", "INBOX"},
		{"Work", "Work"},
		{filepath.Join("Work", "ProjectA"), "Work.ProjectA"},
		{filepath.Join("Work", "ProjectA", "Sub"), "Work.ProjectA.Sub"},
	}
	for _, tt := range tests {
		t.Run(tt.rel, func(t *testing.T) {
			assert.Equal(t, tt.expected, pathToMailbox(tt.rel))
		})
	}
}

// ---- dirHasEML tests ----

func TestDirHasEML(t *testing.T) {
	dir := t.TempDir()

	ok, err := dirHasEML(dir)
	require.NoError(t, err)
	assert.False(t, ok)

	writeEML(t, dir, "msg.eml", "From: a@b.com\r\n\r\nbody")

	ok, err = dirHasEML(dir)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestDirHasEML_UppercaseExtension(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MSG.EML"), []byte("test"), 0o644))
	ok, err := dirHasEML(dir)
	require.NoError(t, err)
	assert.True(t, ok)
}

// ---- collectFolders tests ----

func TestCollectFolders_Empty(t *testing.T) {
	dir := t.TempDir()
	im := New(dir, nil)
	folders, err := im.collectFolders()
	require.NoError(t, err)
	assert.Empty(t, folders)
}

func TestCollectFolders_InboxFromRoot(t *testing.T) {
	dir := t.TempDir()
	writeEML(t, dir, "msg.eml", "From: a@b.com\r\n\r\nbody")

	im := New(dir, nil)
	folders, err := im.collectFolders()
	require.NoError(t, err)
	require.Len(t, folders, 1)
	assert.Equal(t, "INBOX", folders[0].mailbox)
	assert.Equal(t, dir, folders[0].dir)
}

func TestCollectFolders_SingleSubfolder(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "Sent")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	writeEML(t, sub, "msg.eml", "From: a@b.com\r\n\r\nbody")

	im := New(dir, nil)
	folders, err := im.collectFolders()
	require.NoError(t, err)
	require.Len(t, folders, 1)
	assert.Equal(t, "Sent", folders[0].mailbox)
}

func TestCollectFolders_NestedFolders(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, "Work")
	proj := filepath.Join(work, "ProjectA")
	require.NoError(t, os.MkdirAll(proj, 0o755))
	writeEML(t, work, "w.eml", "From: a@b.com\r\n\r\nbody")
	writeEML(t, proj, "p.eml", "From: a@b.com\r\n\r\nbody")

	im := New(dir, nil)
	folders, err := im.collectFolders()
	require.NoError(t, err)

	// Collect mailbox names for assertion order-independent check.
	names := make([]string, len(folders))
	for i, f := range folders {
		names[i] = f.mailbox
	}
	assert.Contains(t, names, "Work")
	assert.Contains(t, names, "Work.ProjectA")
}

func TestCollectFolders_SkipsEmptyDirs(t *testing.T) {
	dir := t.TempDir()
	// Dir with a non-.eml file
	sub := filepath.Join(dir, "Drafts")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "readme.txt"), []byte("hi"), 0o644))

	im := New(dir, nil)
	folders, err := im.collectFolders()
	require.NoError(t, err)
	assert.Empty(t, folders)
}

// ---- Import tests ----

func TestImport_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	client := &mockImportClient{}

	im := New(dir, nil)
	require.NoError(t, im.Import(context.Background(), client))
	assert.Empty(t, client.ensuredMailboxes)
	assert.Empty(t, client.appendedMessages)
}

func TestImport_SingleMessage(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "INBOX")
	require.NoError(t, os.MkdirAll(inbox, 0o755))
	body := "From: sender@example.com\r\nTo: recv@example.com\r\nSubject: Hello\r\n\r\nTest body"
	writeEML(t, inbox, "msg.eml", body)

	client := &mockImportClient{}
	im := New(dir, nil)
	require.NoError(t, im.Import(context.Background(), client))

	assert.Equal(t, []string{"INBOX"}, client.ensuredMailboxes)
	require.Len(t, client.appendedMessages, 1)
	assert.Equal(t, "INBOX", client.appendedMessages[0].folder)
	assert.Equal(t, []byte(body), client.appendedMessages[0].raw)
}

func TestImport_MultipleMessagesInFolder(t *testing.T) {
	dir := t.TempDir()
	sent := filepath.Join(dir, "Sent")
	require.NoError(t, os.MkdirAll(sent, 0o755))
	writeEML(t, sent, "01.eml", "From: a@b.com\r\n\r\nfirst")
	writeEML(t, sent, "02.eml", "From: a@b.com\r\n\r\nsecond")

	client := &mockImportClient{}
	im := New(dir, nil)
	require.NoError(t, im.Import(context.Background(), client))

	assert.Equal(t, []string{"Sent"}, client.ensuredMailboxes)
	assert.Len(t, client.appendedMessages, 2)
	for _, msg := range client.appendedMessages {
		assert.Equal(t, "Sent", msg.folder)
	}
}

func TestImport_NestedFolders(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, "Work")
	proj := filepath.Join(work, "ProjectA")
	require.NoError(t, os.MkdirAll(proj, 0o755))
	writeEML(t, work, "w.eml", "From: a@b.com\r\n\r\nwork")
	writeEML(t, proj, "p.eml", "From: a@b.com\r\n\r\nproject")

	client := &mockImportClient{}
	im := New(dir, nil)
	require.NoError(t, im.Import(context.Background(), client))

	assert.Len(t, client.appendedMessages, 2)

	folders := make(map[string]bool)
	for _, msg := range client.appendedMessages {
		folders[msg.folder] = true
	}
	assert.True(t, folders["Work"])
	assert.True(t, folders["Work.ProjectA"])
}

func TestImport_ProgressCallback(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "INBOX")
	require.NoError(t, os.MkdirAll(inbox, 0o755))
	writeEML(t, inbox, "01.eml", "From: a@b.com\r\n\r\nbody1")
	writeEML(t, inbox, "02.eml", "From: a@b.com\r\n\r\nbody2")

	var updates []export.ProgressUpdate
	cb := func(u export.ProgressUpdate) { updates = append(updates, u) }

	client := &mockImportClient{}
	im := New(dir, cb)
	require.NoError(t, im.Import(context.Background(), client))

	require.Len(t, updates, 2)
	assert.Equal(t, "INBOX", updates[0].Folder)
	assert.Equal(t, 1, updates[0].Current)
	assert.Equal(t, 2, updates[0].Total)
	assert.Equal(t, 2, updates[1].Current)
	assert.Equal(t, 2, updates[1].Total)
	assert.Greater(t, updates[1].BytesTransferred, updates[0].BytesTransferred)
}

func TestImport_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	// Create many folders so we have a chance to cancel mid-way.
	for i := 0; i < 5; i++ {
		sub := filepath.Join(dir, strings.Repeat("A", i+1))
		require.NoError(t, os.MkdirAll(sub, 0o755))
		writeEML(t, sub, "msg.eml", "From: a@b.com\r\n\r\nbody")
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately.
	cancel()

	client := &mockImportClient{}
	im := New(dir, nil)
	err := im.Import(ctx, client)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestImport_EnsureMailboxError(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "INBOX")
	require.NoError(t, os.MkdirAll(inbox, 0o755))
	writeEML(t, inbox, "msg.eml", "From: a@b.com\r\n\r\nbody")

	client := &mockImportClient{ensureErr: errors.New("server error")}
	im := New(dir, nil)
	err := im.Import(context.Background(), client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server error")
}

func TestImport_AppendError(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "INBOX")
	require.NoError(t, os.MkdirAll(inbox, 0o755))
	writeEML(t, inbox, "msg.eml", "From: a@b.com\r\n\r\nbody")

	client := &mockImportClient{appendErr: errors.New("append failed")}
	im := New(dir, nil)
	err := im.Import(context.Background(), client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "append failed")
}

func TestImport_SkipsNonEMLFiles(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "INBOX")
	require.NoError(t, os.MkdirAll(inbox, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "notes.txt"), []byte("ignore me"), 0o644))
	writeEML(t, inbox, "msg.eml", "From: a@b.com\r\n\r\nbody")

	client := &mockImportClient{}
	im := New(dir, nil)
	require.NoError(t, im.Import(context.Background(), client))

	assert.Len(t, client.appendedMessages, 1)
}

func TestImport_EnsureMailboxCalledOncePerFolder(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "INBOX")
	require.NoError(t, os.MkdirAll(inbox, 0o755))
	writeEML(t, inbox, "01.eml", "From: a@b.com\r\n\r\nfirst")
	writeEML(t, inbox, "02.eml", "From: a@b.com\r\n\r\nsecond")
	writeEML(t, inbox, "03.eml", "From: a@b.com\r\n\r\nthird")

	client := &mockImportClient{}
	im := New(dir, nil)
	require.NoError(t, im.Import(context.Background(), client))

	// EnsureMailbox should be called exactly once for INBOX.
	assert.Equal(t, []string{"INBOX"}, client.ensuredMailboxes)
	assert.Len(t, client.appendedMessages, 3)
}

func TestImport_MessageDatePreserved(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "INBOX")
	require.NoError(t, os.MkdirAll(inbox, 0o755))

	emlPath := filepath.Join(inbox, "msg.eml")
	require.NoError(t, os.WriteFile(emlPath, []byte("From: a@b.com\r\n\r\nbody"), 0o644))
	// Set a known mtime.
	known := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(emlPath, known, known))

	client := &mockImportClient{}
	im := New(dir, nil)
	require.NoError(t, im.Import(context.Background(), client))

	require.Len(t, client.appendedMessages, 1)
	// The date should be within 1 second of the known mtime.
	assert.WithinDuration(t, known, client.appendedMessages[0].date, time.Second)
}

// TestImport_RoundTrip verifies that data written by the exporter can be
// faithfully imported back.
func TestImport_RoundTrip(t *testing.T) {
	// Simulate what the exporter writes.
	dir := t.TempDir()

	type fileEntry struct {
		relDir  string
		name    string
		content string
	}
	files := []fileEntry{
		{"INBOX", "00001_2024-01-15_hello.eml", "From: a@b.com\r\nSubject: Hello\r\n\r\nHello body"},
		{"INBOX", "00002_2024-01-16_world.eml", "From: a@b.com\r\nSubject: World\r\n\r\nWorld body"},
		{filepath.Join("Work", "ProjectA"), "00001_2024-02-01_task.eml", "From: a@b.com\r\nSubject: Task\r\n\r\nTask body"},
		{"Sent", "00001_2024-01-20_re-hello.eml", "From: me@b.com\r\nSubject: Re: Hello\r\n\r\nReply body"},
	}

	for _, f := range files {
		subDir := filepath.Join(dir, f.relDir)
		require.NoError(t, os.MkdirAll(subDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, f.name), []byte(f.content), 0o644))
	}

	client := &mockImportClient{}
	im := New(dir, nil)
	require.NoError(t, im.Import(context.Background(), client))

	// All 4 messages should have been appended.
	assert.Len(t, client.appendedMessages, 4)

	// Collect per-folder counts.
	folderCounts := make(map[string]int)
	for _, msg := range client.appendedMessages {
		folderCounts[msg.folder]++
	}
	assert.Equal(t, 2, folderCounts["INBOX"])
	assert.Equal(t, 1, folderCounts["Work.ProjectA"])
	assert.Equal(t, 1, folderCounts["Sent"])

	// Each folder should have been ensured exactly once.
	ensured := make(map[string]int)
	for _, mb := range client.ensuredMailboxes {
		ensured[mb]++
	}
	assert.Equal(t, 1, ensured["INBOX"])
	assert.Equal(t, 1, ensured["Work.ProjectA"])
	assert.Equal(t, 1, ensured["Sent"])
}

func TestImport_BytesAccumulate(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "INBOX")
	require.NoError(t, os.MkdirAll(inbox, 0o755))
	body1 := strings.Repeat("X", 100)
	body2 := strings.Repeat("Y", 200)
	writeEML(t, inbox, "01.eml", body1)
	writeEML(t, inbox, "02.eml", body2)

	var lastBytes int64
	cb := func(u export.ProgressUpdate) { lastBytes = u.BytesTransferred }

	client := &mockImportClient{}
	im := New(dir, cb)
	require.NoError(t, im.Import(context.Background(), client))

	assert.Equal(t, int64(len(body1)+len(body2)), lastBytes)
}
