package model

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func GetAutoRouteScoresForModels(modelNames []string) map[string]int {
	result := make(map[string]int)
	if len(modelNames) == 0 {
		return result
	}

	modelSet := make(map[string]struct{}, len(modelNames))
	for _, name := range modelNames {
		name = strings.TrimSpace(name)
		if name == "" || name == constant.AutoRouteModelName {
			continue
		}
		modelSet[name] = struct{}{}
	}
	if len(modelSet) == 0 {
		return result
	}

	var metas []Model
	if err := DB.Where("status = ? AND router_score > ?", 1, 0).Find(&metas).Error; err != nil {
		return result
	}

	exact := make(map[string]int)
	prefix := make([]Model, 0)
	suffix := make([]Model, 0)
	contains := make([]Model, 0)
	for _, meta := range metas {
		if meta.RouterScore <= 0 {
			continue
		}
		switch meta.NameRule {
		case NameRuleExact:
			if meta.ModelName != "" && meta.RouterScore > exact[meta.ModelName] {
				exact[meta.ModelName] = meta.RouterScore
			}
		case NameRulePrefix:
			prefix = append(prefix, meta)
		case NameRuleSuffix:
			suffix = append(suffix, meta)
		case NameRuleContains:
			contains = append(contains, meta)
		}
	}

	for modelName := range modelSet {
		if score := exact[modelName]; score > 0 {
			result[modelName] = clampAutoRouteScore(score)
			continue
		}
		if score := bestRuleScore(modelName, prefix, func(name, rule string) bool {
			return strings.HasPrefix(name, rule)
		}); score > 0 {
			result[modelName] = clampAutoRouteScore(score)
			continue
		}
		if score := bestRuleScore(modelName, suffix, func(name, rule string) bool {
			return strings.HasSuffix(name, rule)
		}); score > 0 {
			result[modelName] = clampAutoRouteScore(score)
			continue
		}
		if score := bestRuleScore(modelName, contains, func(name, rule string) bool {
			return strings.Contains(name, rule)
		}); score > 0 {
			result[modelName] = clampAutoRouteScore(score)
		}
	}

	return result
}

func HasUsableAutoRouteCandidates(modelNames []string) bool {
	scores := GetAutoRouteScoresForModels(modelNames)
	for modelName, score := range scores {
		if score <= 0 {
			continue
		}
		if IsModelPriced(modelName) {
			return true
		}
	}
	return false
}

func IsModelPriced(modelName string) bool {
	if modelName == "" {
		return false
	}
	if _, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return true
	}
	if billing_setting.GetBillingMode(modelName) == billing_setting.BillingModeTieredExpr {
		if expr, ok := billing_setting.GetBillingExpr(modelName); ok && strings.TrimSpace(expr) != "" {
			return true
		}
	}
	_, ok, _ := ratio_setting.GetModelRatio(modelName)
	return ok
}

func clampAutoRouteScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func bestRuleScore(modelName string, rules []Model, match func(string, string) bool) int {
	best := 0
	for _, rule := range rules {
		ruleName := strings.TrimSpace(rule.ModelName)
		if ruleName == "" || !match(modelName, ruleName) {
			continue
		}
		if rule.RouterScore > best {
			best = rule.RouterScore
		}
	}
	return best
}
