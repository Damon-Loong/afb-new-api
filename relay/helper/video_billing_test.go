package helper

import (
	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestPrepareVideoBilling(t *testing.T) {
	cfg := config.GlobalConfig.Get("billing_setting")
	oldModes := billing_setting.GetBillingModeCopy()
	oldJSON, _ := common.Marshal(oldModes)
	t.Cleanup(func() {
		config.UpdateConfigFromMap(cfg, map[string]string{"billing_mode": string(oldJSON), "video_pricing": "{}"})
	})
	config.UpdateConfigFromMap(cfg, map[string]string{
		"billing_mode":  `{"seedance-video-test":"video_tokens"}`,
		"video_pricing": `{"seedance-video-test":{"exchange_rate":7,"low_no_video":70,"low_with_video":42,"high_no_video":77,"high_with_video":46,"reserve_tokens":300000}}`,
	})
	for _, tc := range []struct {
		body       string
		resolution string
		video      bool
		fail       bool
	}{
		{`{"content":[{"type":"text"}]}`, "720p", false, false},
		{`{"resolution":"1080p","content":[{"type":"video_url"}]}`, "1080p", true, false},
		{`{"draft":true,"resolution":"1080p"}`, "480p", false, false},
		{`{"resolution":"4k"}`, "", false, true},
		{`{"service_tier":"flex"}`, "", false, true},
		{`{"content":[{"type":"draft_task"}]}`, "", false, true},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		info := &relaycommon.RelayInfo{OriginModelName: "seedance-video-test", UsingGroup: "default", UserGroup: "default"}
		err := PrepareVideoBilling(c, info, []byte(tc.body))
		if tc.fail {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, tc.resolution, info.VideoBilling.Resolution)
		require.Equal(t, tc.video, info.VideoBilling.HasVideoInput)
		price, err := ModelPriceHelperPerCall(c, info)
		require.NoError(t, err)
		info.PriceData = price
		ApplyTaskBillingRatios(info, map[string]float64{"video_input": 0.5})
		require.Equal(t, price.Quota, info.PriceData.Quota, "must not discount twice")
	}
}
