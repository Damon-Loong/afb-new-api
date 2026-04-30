package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const jimengActivityListURL = "https://jimeng.jianying.com/jsonp/mweb/v1/get_weekly_challenge_list?_callback=_replaceWaterfallFeeds"

var marketSourceSyncRunning atomic.Bool

type jimengActivityListResponse struct {
	Data struct {
		Activities []jimengSourceActivity `json:"act_info_list"`
	} `json:"data"`
}

type jimengSourceActivity struct {
	Cover     string            `json:"act_cover"`
	CoverMap  map[string]string `json:"act_cover_map"`
	EndTime   string            `json:"act_end_time"`
	IntroHTML string            `json:"act_introduction"`
	Key       string            `json:"act_key"`
	Name      string            `json:"act_name"`
	Reward    struct {
		Other  string `json:"other"`
		Point  int    `json:"point"`
		Point2 string `json:"point2"`
	} `json:"act_reward"`
	RuleHTML  string   `json:"act_rule"`
	StartTime string   `json:"act_start_time"`
	Status    string   `json:"act_status"`
	SubmitNum int      `json:"act_submit_work_num"`
	Visible   bool     `json:"act_visible"`
	WorkTypes []string `json:"act_work_type_list"`
}

func StartMarketSourceSyncTask() {
	if !common.GetEnvOrDefaultBool("MARKET_JIMENG_SYNC_ENABLED", false) {
		return
	}
	interval := parseDurationEnv("MARKET_JIMENG_SYNC_INTERVAL", 6*time.Hour)
	limit := common.GetEnvOrDefault("MARKET_JIMENG_SYNC_LIMIT", 0)
	go func() {
		ctx := context.Background()
		logger.LogInfo(ctx, fmt.Sprintf("market source sync task started: source=jimeng interval=%s limit=%d", interval, limit))
		time.Sleep(30 * time.Second)
		runMarketSourceSync(ctx, limit)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			runMarketSourceSync(ctx, limit)
		}
	}()
}

func runMarketSourceSync(parent context.Context, limit int) {
	if !marketSourceSyncRunning.CompareAndSwap(false, true) {
		logger.LogInfo(parent, "market source sync skipped: previous run still active")
		return
	}
	defer marketSourceSyncRunning.Store(false)

	ctx, cancel := context.WithTimeout(parent, parseDurationEnv("MARKET_JIMENG_SYNC_TIMEOUT", 45*time.Second))
	defer cancel()

	imported, err := SyncJimengMarketActivities(ctx, limit)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("market source sync failed: source=jimeng error=%v", err))
		return
	}
	logger.LogInfo(ctx, fmt.Sprintf("market source sync finished: source=jimeng imported=%d", imported))
}

func SyncJimengMarketActivities(ctx context.Context, limit int) (int, error) {
	activities, err := fetchJimengSourceActivities(ctx, limit)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, item := range activities {
		if strings.TrimSpace(item.Key) == "" || strings.TrimSpace(item.Name) == "" {
			continue
		}
		activity := jimengToMarketActivity(item)
		var existing model.MarketActivity
		err := model.DB.Where("source_name = ? AND source_key = ?", activity.SourceName, activity.SourceKey).First(&existing).Error
		if err == nil {
			activity.ID = existing.ID
			activity.CreatedBy = existing.CreatedBy
			if existing.SubmissionCount > activity.SubmissionCount {
				activity.SubmissionCount = existing.SubmissionCount
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return imported, err
		}
		if err := model.SaveMarketActivity(&activity); err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}

func fetchJimengSourceActivities(ctx context.Context, limit int) ([]jimengSourceActivity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jimengActivityListURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://jimeng.jianying.com/ai-tool/home?activeTab=activity")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jimeng list status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	payload, err := parseJimengJSONP(string(body))
	if err != nil {
		return nil, err
	}
	var parsed jimengActivityListResponse
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return nil, err
	}
	list := parsed.Data.Activities
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func jimengToMarketActivity(item jimengSourceActivity) model.MarketActivity {
	start := parseJimengTime(item.StartTime)
	end := parseJimengTime(item.EndTime)
	return model.MarketActivity{
		Title:               strings.TrimSpace(item.Name),
		Subtitle:            htmlToPlainText(item.IntroHTML),
		PrizeSummary:        jimengPrizeSummary(item),
		DetailContent:       buildJimengDetailContent(item),
		ExternalURL:         "https://jimeng.jianying.com/ai-tool/activity-detail/" + url.PathEscape(item.Key),
		SourceName:          "jimeng",
		SourceKey:           item.Key,
		SourceStatus:        item.Status,
		CoverURL:            bestJimengCover(item),
		Status:              model.MarketActivityStatusPublished,
		Category:            jimengCategory(item.WorkTypes),
		SubmissionCount:     item.SubmitNum,
		StartTime:           start,
		EndTime:             end,
		SubmissionStartTime: start,
		SubmissionEndTime:   end,
	}
}

func parseJimengJSONP(input string) (string, error) {
	start := strings.Index(input, "{")
	end := strings.LastIndex(input, "};")
	if start < 0 || end < start {
		return "", fmt.Errorf("invalid jimeng jsonp payload")
	}
	return input[start : end+1], nil
}

func buildJimengDetailContent(item jimengSourceActivity) string {
	parts := []string{}
	if intro := htmlToPlainText(item.IntroHTML); intro != "" {
		parts = append(parts, "## 活动简介\n\n"+intro)
	}
	if rule := htmlToMarkdown(cleanJimengHTML(item.RuleHTML)); rule != "" {
		parts = append(parts, rule)
	}
	return cleanImportedDetailText(strings.Join(parts, "\n\n"))
}

func cleanJimengHTML(input string) string {
	for _, pattern := range []string{
		`(?is)<button[^>]*>[\s\S]*?(去创作|立即创作|开始创作)[\s\S]*?</button>`,
		`(?is)<a[^>]*>[\s\S]*?(去创作|立即创作|开始创作)[\s\S]*?</a>`,
	} {
		input = regexp.MustCompile(pattern).ReplaceAllString(input, "")
	}
	return input
}

func htmlToMarkdown(input string) string {
	s := html.UnescapeString(input)
	s = regexp.MustCompile(`(?is)<img[^>]+src=["']([^"']+)["'][^>]*>`).ReplaceAllString(s, "\n\n![]($1)\n\n")
	s = htmlLinksToMarkdown(s)
	s = regexp.MustCompile(`(?is)<br\s*/?>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`(?is)</(p|div|section|li|h[1-6])>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`(?is)<strong[^>]*>(.*?)</strong>`).ReplaceAllString(s, "**$1**")
	s = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = regexp.MustCompile(`[ \t]+\n`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func htmlLinksToMarkdown(input string) string {
	linkRe := regexp.MustCompile(`(?is)<a[^>]+href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	return linkRe.ReplaceAllStringFunc(input, func(match string) string {
		parts := linkRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		href := strings.TrimSpace(html.UnescapeString(parts[1]))
		label := htmlToPlainTextWithoutLinks(parts[2])
		if href == "" || label == "" {
			return label
		}
		return "[" + label + "](" + normalizeJimengURL(href) + ")"
	})
}

func htmlToPlainTextWithoutLinks(input string) string {
	text := html.UnescapeString(input)
	text = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func htmlToPlainText(input string) string {
	text := htmlToMarkdown(input)
	text = regexp.MustCompile(`!\[[^\]]*]\([^)]+\)`).ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "**", "")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func cleanImportedDetailText(input string) string {
	replacements := []struct {
		old string
		new string
	}{
		{"并点击本活动页右上角的「立即投稿」;", "并按活动要求完成投稿;"},
		{"并点击本活动页右上角的「立即投稿」；", "并按活动要求完成投稿；"},
		{"并点击本活动页右上角的“立即投稿”;", "并按活动要求完成投稿;"},
		{"并点击本活动页右上角的“立即投稿”；", "并按活动要求完成投稿；"},
		{"点击本活动页右上角的「立即投稿」", "按活动要求完成投稿"},
		{"点击本活动页右上角的“立即投稿”", "按活动要求完成投稿"},
		{"点击本活动页右上角的「去创作」", "按活动要求完成创作"},
		{"点击本活动页右上角的“去创作”", "按活动要求完成创作"},
		{"去创作", "创作"},
		{"立即创作", "创作"},
		{"开始创作", "创作"},
	}
	for _, item := range replacements {
		input = strings.ReplaceAll(input, item.old, item.new)
	}
	return strings.TrimSpace(input)
}

func jimengPrizeSummary(item jimengSourceActivity) string {
	if strings.TrimSpace(item.Reward.Other) != "" {
		return strings.TrimSpace(item.Reward.Other)
	}
	if strings.TrimSpace(item.Reward.Point2) != "" {
		return strings.TrimSpace(item.Reward.Point2)
	}
	if item.Reward.Point > 0 {
		return "本期活动设置 " + strconv.Itoa(item.Reward.Point) + " 即梦积分"
	}
	return ""
}

func bestJimengCover(item jimengSourceActivity) string {
	for _, key := range []string{"1080", "720", "480", "360", "origin"} {
		if item.CoverMap != nil && strings.TrimSpace(item.CoverMap[key]) != "" {
			return item.CoverMap[key]
		}
	}
	return item.Cover
}

func jimengCategory(types []string) string {
	for _, typ := range types {
		switch typ {
		case "short_video", "video":
			return "video"
		case "image":
			return "image"
		case "music", "audio":
			return "music"
		case "text", "document":
			return "text"
		}
	}
	return "mixed"
}

func normalizeJimengURL(href string) string {
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return "https://jimeng.jianying.com" + href
	}
	return href
}

func parseJimengTime(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, layout := range []string{"2006-01-02 15:04:05", "2006.1.2 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t.Unix()
		}
	}
	return 0
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		common.SysError(fmt.Sprintf("failed to parse %s=%q, using default %s", key, value, fallback))
		return fallback
	}
	return duration
}
