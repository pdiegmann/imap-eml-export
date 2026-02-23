package imapclient

import (
	"crypto/tls"
	"fmt"
	"io"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/pdiegmann/imap-eml-export/internal/export"
)

// isAlreadyExistsErr returns true when the IMAP server rejected CREATE because
// the mailbox already exists (RFC 9051 §7.1 [ALREADYEXISTS] response code, or
// servers that send a plain NO with similar wording).
func isAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	if imapErr, ok := err.(*imap.Error); ok {
		return imapErr.Code == imap.ResponseCodeAlreadyExists
	}
	return false
}

// Client wraps the go-imap/v2 client.
type Client struct {
	addr     string
	username string
	password string
	useTLS   bool
	startTLS bool
	c        *imapclient.Client
}

// New creates a new IMAP client.
func New(host string, port int, username, password string, useTLS, startTLS bool) *Client {
	return &Client{
		addr:     fmt.Sprintf("%s:%d", host, port),
		username: username,
		password: password,
		useTLS:   useTLS,
		startTLS: startTLS,
	}
}

// Connect establishes the network connection.
func (c *Client) Connect() error {
	opts := &imapclient.Options{
		TLSConfig: &tls.Config{},
	}

	var (
		client *imapclient.Client
		err    error
	)

	switch {
	case c.startTLS:
		client, err = imapclient.DialStartTLS(c.addr, opts)
	case c.useTLS:
		client, err = imapclient.DialTLS(c.addr, opts)
	default:
		client, err = imapclient.DialInsecure(c.addr, opts)
	}
	if err != nil {
		return fmt.Errorf("dialing %s: %w", c.addr, err)
	}
	c.c = client
	return nil
}

// Authenticate logs in with IMAP credentials.
func (c *Client) Authenticate() error {
	if err := c.c.Login(c.username, c.password).Wait(); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	return nil
}

// ListFolders returns all mailbox names.
func (c *Client) ListFolders() ([]string, error) {
	listCmd := c.c.List("", "*", nil)
	mailboxes, err := listCmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("listing mailboxes: %w", err)
	}

	names := make([]string, 0, len(mailboxes))
	for _, mb := range mailboxes {
		names = append(names, mb.Mailbox)
	}
	return names, nil
}

// FetchMessages fetches all messages from the given folder.
func (c *Client) FetchMessages(folder string) ([]export.Message, error) {
	selectData, err := c.c.Select(folder, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("selecting folder %q: %w", folder, err)
	}

	if selectData.NumMessages == 0 {
		return nil, nil
	}

	// Build a SeqSet covering all messages: 1:*
	var seqSet imap.SeqSet
	seqSet.AddRange(1, 0) // 0 means *

	bodySection := &imap.FetchItemBodySection{}
	fetchOpts := &imap.FetchOptions{
		Envelope:     true,
		InternalDate: true,
		UID:          true,
		BodySection:  []*imap.FetchItemBodySection{bodySection},
	}

	fetchCmd := c.c.Fetch(seqSet, fetchOpts)
	defer fetchCmd.Close()

	var messages []export.Message
	for {
		msgData := fetchCmd.Next()
		if msgData == nil {
			break
		}

		buf, err := msgData.Collect()
		if err != nil {
			return nil, fmt.Errorf("collecting message data: %w", err)
		}

		msg := export.Message{
			UID:  uint32(buf.UID),
			Date: buf.InternalDate,
		}

		if buf.Envelope != nil {
			msg.Subject = buf.Envelope.Subject
			if !buf.Envelope.Date.IsZero() {
				msg.Date = buf.Envelope.Date
			}
		}
		if msg.Date.IsZero() {
			msg.Date = time.Now()
		}

		// Retrieve raw body bytes from the first body section.
		for _, raw := range buf.BodySection {
			if raw != nil {
				msg.Raw = raw
			}
		}
		if msg.Raw == nil {
			msg.Raw = []byte{}
		}

		messages = append(messages, msg)
	}

	if err := fetchCmd.Close(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("closing fetch command: %w", err)
	}

	return messages, nil
}

// EnsureMailbox creates the mailbox on the server if it does not already exist.
// It is idempotent: if the mailbox already exists the call succeeds silently.
func (c *Client) EnsureMailbox(name string) error {
	err := c.c.Create(name, nil).Wait()
	if err == nil || isAlreadyExistsErr(err) {
		return nil
	}
	// Some servers do not send [ALREADYEXISTS] but return a generic NO.
	// Try SELECT to confirm the mailbox is reachable despite the error.
	if _, selectErr := c.c.Select(name, nil).Wait(); selectErr == nil {
		return nil
	}
	return fmt.Errorf("creating mailbox %q: %w", name, err)
}

// AppendMessage uploads raw EML bytes to the named mailbox.
// date is recorded as the IMAP internal date; a zero value lets the server
// choose.
func (c *Client) AppendMessage(folder string, date time.Time, raw []byte) error {
	opts := &imap.AppendOptions{}
	if !date.IsZero() {
		opts.Time = date
	}
	appendCmd := c.c.Append(folder, int64(len(raw)), opts)
	if _, err := appendCmd.Write(raw); err != nil {
		return fmt.Errorf("writing message body: %w", err)
	}
	if err := appendCmd.Close(); err != nil {
		return fmt.Errorf("closing append writer: %w", err)
	}
	if _, err := appendCmd.Wait(); err != nil {
		return fmt.Errorf("append command: %w", err)
	}
	return nil
}

// Close terminates the IMAP connection.
func (c *Client) Close() error {
	if c.c != nil {
		return c.c.Logout().Wait()
	}
	return nil
}
