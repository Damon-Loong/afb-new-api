package doubao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimateNativeBillingUsesVideoInputRatio(t *testing.T) {
	raw := []byte(`{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"test"},{"type":"video_url","video_url":{"url":"https://example.com/input.mp4"}}]}`)

	ratios, err := EstimateNativeBilling(raw, "doubao-seedance-2-0-260128")
	require.NoError(t, err)
	assert.InDelta(t, 28.0/46.0, ratios["video_input"], 0.000001)
}

func TestEstimateNativeBillingIgnoresImageOnlyInput(t *testing.T) {
	raw := []byte(`{"content":[{"type":"image_url","image_url":{"url":"https://example.com/input.png"}}]}`)

	ratios, err := EstimateNativeBilling(raw, "doubao-seedance-2-0-260128")
	require.NoError(t, err)
	assert.Empty(t, ratios)
}
