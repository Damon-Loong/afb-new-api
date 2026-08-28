package helper

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
)

func TestApplyTaskBillingRatios(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "billing-test-model",
		PriceData: types.PriceData{
			Quota: 100,
		},
	}

	ApplyTaskBillingRatios(info, map[string]float64{"video_input": 0.5})

	assert.Equal(t, 50, info.PriceData.Quota)
	assert.Equal(t, 0.5, info.PriceData.OtherRatios["video_input"])
}
