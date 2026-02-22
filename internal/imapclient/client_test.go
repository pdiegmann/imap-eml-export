package imapclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	c := New("imap.example.com", 993, "user@example.com", "secret", true, false)
	assert.NotNil(t, c)
	assert.Equal(t, "imap.example.com:993", c.addr)
	assert.Equal(t, "user@example.com", c.username)
	assert.True(t, c.useTLS)
}

func TestNewClientStartTLS(t *testing.T) {
	c := New("imap.example.com", 143, "user@example.com", "secret", false, true)
	assert.NotNil(t, c)
	assert.True(t, c.startTLS)
	assert.False(t, c.useTLS)
}
