package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func TestTheLogKeepsAppendingBelowTheLimit(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "x.log")
	w, err := Open(path, 100)
	require.NoError(t, err)
	defer w.Close()

	// Act
	_, _ = w.Write([]byte("one\n"))
	_, _ = w.Write([]byte("two\n"))

	// Assert
	assert.Equal(t, "one\ntwo\n", read(t, path))
	assert.NoFileExists(t, path+".1")
}

func TestPassingTheLimitMovesTheLogAsideAndStartsANewOne(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "x.log")
	w, err := Open(path, 20)
	require.NoError(t, err)
	defer w.Close()
	_, _ = w.Write([]byte(strings.Repeat("a", 15) + "\n"))

	// Act
	_, _ = w.Write([]byte("new line\n"))

	// Assert
	assert.Equal(t, "new line\n", read(t, path))
	assert.Equal(t, strings.Repeat("a", 15)+"\n", read(t, path+".1"))
}

func TestOnlyOneOlderLogIsKept(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "x.log")
	w, err := Open(path, 10)
	require.NoError(t, err)
	defer w.Close()

	// Act
	for _, s := range []string{"1111111111\n", "2222222222\n", "3333333333\n"} {
		_, _ = w.Write([]byte(s))
	}

	// Assert
	assert.Equal(t, "3333333333\n", read(t, path))
	assert.Equal(t, "2222222222\n", read(t, path+".1"), "the oldest was dropped")
}

func TestAFileAlreadyOverTheLimitIsRotatedWhenOpened(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "x.log")
	require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("z", 50)), 0o644))

	// Act
	w, err := Open(path, 20)
	require.NoError(t, err)
	defer w.Close()
	_, _ = w.Write([]byte("fresh\n"))

	// Assert
	assert.Equal(t, "fresh\n", read(t, path))
	assert.Equal(t, strings.Repeat("z", 50), read(t, path+".1"))
}
