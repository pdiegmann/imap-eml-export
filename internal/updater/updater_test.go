package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOSArch(t *testing.T) {
	result := OSArch()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "/")
}
