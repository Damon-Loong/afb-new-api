package service

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const testTieredQuotaPerUnit = 500_000.0

func makeTieredRelayInfo(expr string, groupRatio float64, prompt, completion int) *relaycommon.RelayInfo {
	cost, trace, _ := billingexpr.RunExpr(expr, billingexpr.TokenParams{P: float64(prompt), C: float64(completion)})
	beforeGroup := cost / 1_000_000 * testTieredQuotaPerUnit
	afterGroup := billingexpr.QuotaRound(beforeGroup * groupRatio)
	return &relaycommon.RelayInfo{
		FinalPreConsumedQuota: afterGroup,
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:               "tiered_expr",
			ExprString:                expr,
			ExprHash:                  billingexpr.ExprHashString(expr),
			GroupRatio:                groupRatio,
			EstimatedPromptTokens:     prompt,
			EstimatedCompletionTokens: completion,
			EstimatedQuotaBeforeGroup: beforeGroup,
			EstimatedQuotaAfterGroup:  afterGroup,
			EstimatedTier:             trace.MatchedTier,
			QuotaPerUnit:              testTieredQuotaPerUnit,
		},
	}
}

func TestTryTieredSettleUsesFrozenRequestInput(t *testing.T) {
	expr := `param("service_tier") == "fast" ? tier("fast", p * 2) : tier("normal", p)`
	info := makeTieredRelayInfo(expr, 1, 100, 0)
	info.BillingRequestInput = &billingexpr.RequestInput{Body: []byte(`{"service_tier":"fast"}`)}

	ok, quota, result := TryTieredSettle(info, billingexpr.TokenParams{P: 100})
	if !ok || quota != 100 || result == nil || result.MatchedTier != "fast" {
		t.Fatalf("settle = (%v, %d, %#v), want (true, 100, fast)", ok, quota, result)
	}
}

func TestTryTieredSettleFallsBackToFrozenPreConsumeOnExprError(t *testing.T) {
	info := &relaycommon.RelayInfo{
		FinalPreConsumedQuota: 321,
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:              "tiered_expr",
			ExprString:               `invalid +-+ expr`,
			EstimatedQuotaAfterGroup: 123,
		},
	}

	ok, quota, result := TryTieredSettle(info, billingexpr.TokenParams{P: 100})
	if !ok || quota != 321 || result != nil {
		t.Fatalf("settle = (%v, %d, %#v), want frozen quota 321", ok, quota, result)
	}
}

func TestTryTieredSettleTierBoundary(t *testing.T) {
	expr := `p <= 200000 ? tier("standard", p * 1.5 + c * 7.5) : tier("long_context", p * 3 + c * 11.25)`
	info := makeTieredRelayInfo(expr, 1, 200_000, 1_000)

	_, standardQuota, standard := TryTieredSettle(info, billingexpr.TokenParams{P: 200_000, C: 1_000})
	if standardQuota != 153_750 || standard == nil || standard.MatchedTier != "standard" {
		t.Fatalf("boundary settle = (%d, %#v), want standard/153750", standardQuota, standard)
	}
	_, longQuota, long := TryTieredSettle(info, billingexpr.TokenParams{P: 200_001, C: 1_000})
	if longQuota != 305_627 || long == nil || long.MatchedTier != "long_context" || !long.CrossedTier {
		t.Fatalf("boundary+1 settle = (%d, %#v), want long_context/305627", longQuota, long)
	}
}

type recordingTieredBillingSettler struct {
	preConsumedQuota int
	reserveTargets   []int
}

func (*recordingTieredBillingSettler) Settle(int) error           { return nil }
func (*recordingTieredBillingSettler) Refund(*gin.Context)        {}
func (*recordingTieredBillingSettler) NeedsRefund() bool          { return false }
func (s *recordingTieredBillingSettler) GetPreConsumedQuota() int { return s.preConsumedQuota }
func (s *recordingTieredBillingSettler) Reserve(target int) error {
	s.reserveTargets = append(s.reserveTargets, target)
	if target > s.preConsumedQuota {
		s.preConsumedQuota = target
	}
	return nil
}

func TestPrepareTieredBillingForSelectedGroupUpdatesReservation(t *testing.T) {
	billing := &recordingTieredBillingSettler{preConsumedQuota: 50_000}
	info := makeTieredRelayInfo(`tier("base", p)`, 0.1, 1_000_000, 0)
	info.Billing = billing
	info.PriceData = types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.2}}

	if apiErr := PrepareTieredBillingForSelectedGroup(nil, info); apiErr != nil {
		t.Fatal(apiErr)
	}
	if len(billing.reserveTargets) != 1 || billing.reserveTargets[0] != 100_000 {
		t.Fatalf("reserve targets = %v, want [100000]", billing.reserveTargets)
	}
	if info.FinalPreConsumedQuota != 100_000 || info.TieredBillingSnapshot.GroupRatio != 0.2 {
		t.Fatalf("refreshed billing state = quota %d ratio %f", info.FinalPreConsumedQuota, info.TieredBillingSnapshot.GroupRatio)
	}
}

func TestTryTieredSettleUsesFinalGroupAfterRetry(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ratio     float64
		wantQuota int
	}{{"paid", 0.2, 100_000}, {"free", 0, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			info := makeTieredRelayInfo(`tier("base", p)`, 0.1, 1_000_000, 0)
			info.Billing = &recordingTieredBillingSettler{preConsumedQuota: 50_000}
			info.PriceData = types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: tc.ratio}}
			if apiErr := PrepareTieredBillingForSelectedGroup(nil, info); apiErr != nil {
				t.Fatal(apiErr)
			}
			ok, quota, _ := TryTieredSettle(info, billingexpr.TokenParams{P: 1_000_000})
			if !ok || quota != tc.wantQuota {
				t.Fatalf("settle = (%v, %d), want (true, %d)", ok, quota, tc.wantQuota)
			}
		})
	}
}

func tieredNormalizedCost(expr string, usage *dto.Usage, claude bool) float64 {
	params := BuildTieredTokenParams(usage, claude, billingexpr.UsedVars(expr))
	cost, _, _ := billingexpr.RunExpr(expr, params)
	return cost
}

func TestBuildTieredTokenParamsSeparatesUsedTokenClasses(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     1_000,
		CompletionTokens: 600,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 200,
			ImageTokens:  100,
		},
		CompletionTokenDetails: dto.OutputTokenDetails{AudioTokens: 100},
	}
	expr := `tier("base", p * 2 + c * 10 + cr + img * 3 + ao * 20)`
	got := tieredNormalizedCost(expr, usage, false)
	want := 700*2 + 500*10 + 200 + 100*3 + 100*20
	if math.Abs(got-float64(want)) > 1e-6 {
		t.Fatalf("normalized cost = %f, want %d", got, want)
	}
}

func TestBuildTieredTokenParamsKeepsUnusedClassesInBase(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     1_000,
		CompletionTokens: 600,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 200,
			ImageTokens:  100,
		},
		CompletionTokenDetails: dto.OutputTokenDetails{AudioTokens: 100},
	}
	got := tieredNormalizedCost(`tier("base", p * 2 + c * 10)`, usage, false)
	if got != 8_000 {
		t.Fatalf("normalized cost = %f, want 8000", got)
	}
}

func TestBuildTieredTokenParamsLenPreservesFullContext(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens: 300_000,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 250_000,
		},
	}
	params := BuildTieredTokenParams(usage, false, map[string]bool{"cr": true, "len": true})
	if params.P != 50_000 || params.CR != 250_000 || params.Len != 300_000 {
		t.Fatalf("params = %#v, want p=50000 cr=250000 len=300000", params)
	}
}
