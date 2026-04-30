package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const jimengListURL = "https://jimeng.jianying.com/jsonp/mweb/v1/get_weekly_challenge_list?_callback=_replaceWaterfallFeeds"

type jimengListResponse struct {
	Data struct {
		Activities []jimengActivity `json:"act_info_list"`
	} `json:"data"`
}

type jimengActivity struct {
	Cover       string            `json:"act_cover"`
	CoverMap    map[string]string `json:"act_cover_map"`
	Description string            `json:"act_description"`
	EndTime     string            `json:"act_end_time"`
	IntroHTML   string            `json:"act_introduction"`
	Key         string            `json:"act_key"`
	Name        string            `json:"act_name"`
	Reward      struct {
		Other  string `json:"other"`
		Point  int    `json:"point"`
		Point2 string `json:"point2"`
		Type   string `json:"reward_type"`
	} `json:"act_reward"`
	RuleHTML  string   `json:"act_rule"`
	StartTime string   `json:"act_start_time"`
	Status    string   `json:"act_status"`
	SubmitNum int      `json:"act_submit_work_num"`
	WorkTypes []string `json:"act_work_type_list"`
}

func main() {
	limit := flag.Int("limit", 0, "import activity count, 0 means all")
	dbPath := flag.String("sqlite", firstNonEmpty(os.Getenv("SQLITE_PATH"), "market-dev.db?_busy_timeout=30000"), "sqlite path when SQL_DSN is not set")
	dryRun := flag.Bool("dry-run", false, "print parsed activities without saving")
	flag.Parse()

	activities, err := fetchJimengActivities(*limit)
	if err != nil {
		fatal(err)
	}
	if len(activities) == 0 {
		fatal(fmt.Errorf("no jimeng activities found"))
	}

	if *dryRun {
		for _, activity := range activities {
			fmt.Printf("%s | %s | %s | %d chars\n", activity.Key, activity.Name, prizeSummary(activity), len(activity.RuleHTML))
		}
		return
	}

	db, err := openDB(*dbPath)
	if err != nil {
		fatal(err)
	}
	model.DB = db
	if err := db.AutoMigrate(
		&model.MarketActivity{},
		&model.MarketActivityPolicy{},
		&model.MarketSubmission{},
		&model.MarketUpload{},
	); err != nil {
		fatal(err)
	}

	imported := 0
	for _, item := range activities {
		activity := toMarketActivity(item)
		var existing model.MarketActivity
		err := db.Where("source_name = ? AND source_key = ?", activity.SourceName, activity.SourceKey).First(&existing).Error
		if err == nil {
			activity.ID = existing.ID
			activity.CreatedBy = existing.CreatedBy
			activity.SubmissionCount = max(existing.SubmissionCount, activity.SubmissionCount)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			fatal(err)
		}
		if err := model.SaveMarketActivity(&activity); err != nil {
			fatal(err)
		}
		imported++
		fmt.Printf("imported #%d: %s (%s)\n", activity.ID, activity.Title, activity.ExternalURL)
	}
	fmt.Printf("done, imported %d activity(s)\n", imported)
}

func fetchJimengActivities(limit int) ([]jimengActivity, error) {
	req, err := http.NewRequest(http.MethodGet, jimengListURL, nil)
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
	payload, err := parseJSONP(string(body))
	if err != nil {
		return nil, err
	}
	var parsed jimengListResponse
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return nil, err
	}
	list := parsed.Data.Activities
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func parseJSONP(input string) (string, error) {
	start := strings.Index(input, "{")
	end := strings.LastIndex(input, "};")
	if start < 0 || end < start {
		return "", fmt.Errorf("invalid jsonp payload")
	}
	return input[start : end+1], nil
}

func openDB(sqlitePath string) (*gorm.DB, error) {
	dsn := strings.TrimSpace(os.Getenv("SQL_DSN"))
	if dsn == "" {
		return gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	}
	if strings.HasPrefix(dsn, "local") {
		return gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	}
	if !strings.Contains(dsn, "parseTime") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}

func toMarketActivity(item jimengActivity) model.MarketActivity {
	start := parseChinaTime(item.StartTime)
	end := parseChinaTime(item.EndTime)
	return model.MarketActivity{
		Title:               strings.TrimSpace(item.Name),
		Subtitle:            htmlToPlainText(item.IntroHTML),
		PrizeSummary:        prizeSummary(item),
		DetailContent:       buildDetailContent(item),
		ExternalURL:         "https://jimeng.jianying.com/ai-tool/activity-detail/" + url.PathEscape(item.Key),
		SourceName:          "jimeng",
		SourceKey:           item.Key,
		SourceStatus:        item.Status,
		CoverURL:            bestCover(item),
		Status:              model.MarketActivityStatusPublished,
		Category:            category(item.WorkTypes),
		SubmissionCount:     item.SubmitNum,
		StartTime:           start,
		EndTime:             end,
		SubmissionStartTime: start,
		SubmissionEndTime:   end,
	}
}

func buildDetailContent(item jimengActivity) string {
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

func normalizeJimengURL(href string) string {
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return "https://jimeng.jianying.com" + href
	}
	return href
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

func prizeSummary(item jimengActivity) string {
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

func bestCover(item jimengActivity) string {
	for _, key := range []string{"1080", "720", "480", "360", "origin"} {
		if item.CoverMap != nil && strings.TrimSpace(item.CoverMap[key]) != "" {
			return item.CoverMap[key]
		}
	}
	return item.Cover
}

func category(types []string) string {
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

func parseChinaTime(value string) int64 {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
