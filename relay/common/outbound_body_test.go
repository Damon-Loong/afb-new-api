package common

import (
	"bytes"
	"io"
	"testing"

	basecommon "github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestNewOutboundJSONBodyMemoryPath(t *testing.T) {
	oldConfig := basecommon.GetDiskCacheConfig()
	basecommon.SetDiskCacheConfig(basecommon.DiskCacheConfig{
		Enabled:     false,
		ThresholdMB: 10,
		MaxSizeMB:   1024,
		Path:        oldConfig.Path,
	})
	t.Cleanup(func() { basecommon.SetDiskCacheConfig(oldConfig) })

	data := []byte(`{"image":"data:image/png;base64,AAAA"}`)
	body, size, closer, err := NewOutboundJSONBody(data)
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), size)
	require.IsType(t, &bytes.Reader{}, body)
	require.NotNil(t, closer)
	require.NoError(t, closer.Close())

	actual, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, data, actual)
}

func TestNewOutboundJSONBodyReaderKeepsDataAlive(t *testing.T) {
	oldConfig := basecommon.GetDiskCacheConfig()
	basecommon.SetDiskCacheConfig(basecommon.DiskCacheConfig{Enabled: false})
	t.Cleanup(func() { basecommon.SetDiskCacheConfig(oldConfig) })

	data := []byte(`{"model":"gpt-test"}`)
	body, size, closer, err := NewOutboundJSONBody(data)
	require.NoError(t, err)
	data = nil

	actual, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, int64(len(actual)), size)
	require.JSONEq(t, `{"model":"gpt-test"}`, string(actual))
	require.NoError(t, closer.Close())
}
