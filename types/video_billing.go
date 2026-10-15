package types

import (
	"fmt"
	"math"
)

// VideoTokenPricing uses CNY per million provider-reported video tokens.
// ExchangeRate converts CNY to the platform's internal USD quota exactly once.
type VideoTokenPricing struct {
	ExchangeRate  float64 `json:"exchange_rate"`
	LowNoVideo    float64 `json:"low_no_video"`
	LowWithVideo  float64 `json:"low_with_video"`
	HighNoVideo   float64 `json:"high_no_video"`
	HighWithVideo float64 `json:"high_with_video"`
	ReserveTokens int     `json:"reserve_tokens"`
}

func (p VideoTokenPricing) Validate() error {
	for _, v := range []float64{p.ExchangeRate, p.LowNoVideo, p.LowWithVideo, p.HighNoVideo, p.HighWithVideo} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 1000000 {
			return fmt.Errorf("视频单价及汇率必须为大于 0 且不超过 1000000 的数字")
		}
	}
	if p.ReserveTokens <= 0 || p.ReserveTokens > 100000000 {
		return fmt.Errorf("视频预扣 Token 数必须在 1 至 100000000 之间")
	}
	return nil
}

func (p VideoTokenPricing) Price(resolution string, videoInput bool) (float64, error) {
	switch resolution {
	case "480p", "720p":
		if videoInput {
			return p.LowWithVideo, nil
		}
		return p.LowNoVideo, nil
	case "1080p":
		if videoInput {
			return p.HighWithVideo, nil
		}
		return p.HighNoVideo, nil
	default:
		return 0, fmt.Errorf("不支持的视频计费分辨率 %q", resolution)
	}
}

// VideoBillingSnapshot is immutable pricing captured at submission; actual
// output resolution and usage come from the successful provider response.
type VideoBillingSnapshot struct {
	Pricing       VideoTokenPricing `json:"pricing"`
	Resolution    string            `json:"resolution"`
	HasVideoInput bool              `json:"has_video_input"`
	Draft         bool              `json:"draft"`
	GroupRatio    float64           `json:"group_ratio"`
	QuotaPerUnit  float64           `json:"quota_per_unit"`
}

func (s VideoBillingSnapshot) Quota(tokens int, resolution string) (int, error) {
	if s.QuotaPerUnit <= 0 || s.GroupRatio < 0 {
		return 0, fmt.Errorf("无效的视频计费额度单位或分组倍率")
	}
	if err := s.Pricing.Validate(); err != nil {
		return 0, err
	}
	if tokens <= 0 {
		return 0, fmt.Errorf("上游尚未返回有效 completion_tokens，等待核对")
	}
	price, err := s.Pricing.Price(resolution, s.HasVideoInput)
	if err != nil {
		return 0, err
	}
	q := float64(tokens) / 1000000 * price / s.Pricing.ExchangeRate * s.GroupRatio * s.QuotaPerUnit
	if math.IsNaN(q) || math.IsInf(q, 0) || q < 0 || q >= float64(math.MaxInt) {
		return 0, fmt.Errorf("视频额度计算超出范围")
	}
	return int(math.Round(q)), nil
}
