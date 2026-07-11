package common

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestLocalLogPreview(t *testing.T) {
	oldDebug := DebugEnabled
	t.Cleanup(func() { DebugEnabled = oldDebug })

	DebugEnabled = false
	short := "short error"
	require.Equal(t, short, LocalLogPreview(short))

	long := strings.Repeat("错", LocalLogContentLimit)
	preview := LocalLogPreview(long)
	require.True(t, utf8.ValidString(preview))
	require.Contains(t, preview, "[truncated")
	require.Contains(t, preview, "original_length=")

	DebugEnabled = true
	require.Equal(t, long, LocalLogPreview(long))
}

func TestStoredErrorPreviewAlwaysLimitsContent(t *testing.T) {
	content := strings.Repeat("误", StoredErrorContentLimit)
	preview := StoredErrorPreview(content)
	require.True(t, utf8.ValidString(preview))
	require.Contains(t, preview, "[truncated")
	require.Less(t, len(preview), len(content))
}
