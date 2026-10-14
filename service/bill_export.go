package service

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

type BillExportOptions struct {
	model.BillExportFilter
	Format     string   `json:"format"`
	Timezone   string   `json:"timezone"`
	Dimensions []string `json:"dimensions"`
}

var billDimensions = map[string]string{
	"user": "用户汇总", "model": "模型汇总", "day": "每日汇总",
	"token": "令牌汇总", "group": "分组汇总", "channel": "渠道汇总",
}

func (o *BillExportOptions) Validate(admin bool) error {
	if o.StartTimestamp <= 0 || o.EndTimestamp < o.StartTimestamp || o.EndTimestamp > 253402300799 {
		return errors.New("请选择有效的账单起止时间")
	}
	if o.EndTimestamp-o.StartTimestamp > 366*24*60*60 {
		return errors.New("单次账单周期不能超过 366 天")
	}
	if o.LogType != 0 && !model.IsBillLogType(o.LogType) {
		return errors.New("消费账单仅包含消费和额度退回，请调整日志类型")
	}
	if o.Channel < 0 {
		return errors.New("渠道 ID 无效")
	}
	if o.Format != "xlsx" && o.Format != "csv" {
		return errors.New("请选择 Excel 或 CSV 格式")
	}
	if o.Timezone == "" {
		o.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(o.Timezone); err != nil {
		return errors.New("账单时区无效")
	}
	if len(o.Dimensions) > len(billDimensions) {
		return errors.New("汇总维度无效")
	}
	seen := make(map[string]bool)
	for _, dimension := range o.Dimensions {
		if _, ok := billDimensions[dimension]; !ok || seen[dimension] {
			return errors.New("汇总维度无效或重复")
		}
		if dimension == "channel" && !admin {
			return errors.New("无权导出渠道维度")
		}
		seen[dimension] = true
	}
	if !admin {
		o.Username = ""
		o.Channel = 0
	}
	return nil
}

type billMetadata struct {
	BillingSource       string `json:"billing_source"`
	PlanTitle           string `json:"subscription_plan_title"`
	SubscriptionID      int64  `json:"subscription_id"`
	TaskID              string `json:"task_id"`
	CacheTokens         *int64 `json:"cache_tokens"`
	CacheCreationTokens *int64 `json:"cache_creation_tokens"`
}

type billEntry struct {
	log    model.Log
	meta   billMetadata
	source string
	// Wallet, subscription, and unclassified deductions/refunds in raw quota.
	amounts [6]decimal.Decimal
}

func makeBillEntry(log model.Log) billEntry {
	e := billEntry{log: log, source: "未记录"}
	// Parse only explicitly permitted fields. Never export raw Other/admin_info.
	if err := json.Unmarshal([]byte(log.Other), &e.meta); err != nil {
		e.meta = billMetadata{}
	}
	index := 4
	switch {
	case log.Type == model.LogTypeMarketConsume || log.Type == model.LogTypeProjectConsume:
		// These business flows charge the wallet directly. Project metadata can
		// be supplied by callers and must not override the actual funding source.
		e.source, index = "钱包", 0
	case e.meta.BillingSource == "subscription":
		e.source, index = "订阅", 2
	case e.meta.BillingSource == "wallet":
		e.source, index = "钱包", 0
	}
	quota := decimal.NewFromInt(int64(log.Quota))
	if log.Type == model.LogTypeRefund || quota.IsNegative() {
		index++
	}
	e.amounts[index] = quota.Abs()
	return e
}

type billTotals struct {
	entries            int
	consumeEntries     int
	prompt, completion decimal.Decimal
	amounts            [6]decimal.Decimal
}

func (b *billTotals) add(e billEntry) {
	b.entries++
	if model.IsConsumeLogType(e.log.Type) {
		b.consumeEntries++
	}
	b.prompt = b.prompt.Add(decimal.NewFromInt(int64(e.log.PromptTokens)))
	b.completion = b.completion.Add(decimal.NewFromInt(int64(e.log.CompletionTokens)))
	for i, amount := range e.amounts {
		b.amounts[i] = b.amounts[i].Add(amount)
	}
}

func billNumber(value decimal.Decimal) interface{} {
	// Excel only preserves 15 significant decimal digits. Keep unusually large
	// values as text instead of silently corrupting the original quota.
	if value.Abs().GreaterThanOrEqual(decimal.New(1, 15)) {
		return value.String()
	}
	f, _ := value.Float64()
	return f
}

func billAmountCells(amounts [6]decimal.Decimal, quotaPerUnit decimal.Decimal) []interface{} {
	result := make([]interface{}, 0, 11)
	for _, amount := range amounts {
		result = append(result, billNumber(amount))
	}
	result = append(result,
		billNumber(amounts[0].Sub(amounts[1])),
		billNumber(amounts[2].Sub(amounts[3])),
		billNumber(amounts[0].Div(quotaPerUnit).Round(10)),
		billNumber(amounts[1].Div(quotaPerUnit).Round(10)),
		billNumber(amounts[0].Sub(amounts[1]).Div(quotaPerUnit).Round(10)),
	)
	return result
}

var billAmountHeaders = []interface{}{
	"钱包扣除额度", "钱包退回额度", "订阅消耗额度", "订阅退回额度", "来源未记录扣除额度", "来源未记录退回额度",
	"钱包净扣额度", "订阅净消耗额度", "钱包扣费折算(USD)", "钱包退回折算(USD)", "钱包净扣折算(USD)",
}

type billSheet struct {
	name    string
	headers []interface{}
	rows    func(func([]interface{}) error) error
}

func billTypeName(logType int) string {
	switch logType {
	case model.LogTypeConsume:
		return "API/任务消费"
	case model.LogTypeRefund:
		return "额度退回"
	case model.LogTypeMarketConsume:
		return "市场消耗"
	case model.LogTypeProjectConsume:
		return "项目消耗"
	default:
		return "全部消费及额度退回"
	}
}

func billDimension(e billEntry, dimension string, location *time.Location) (string, string) {
	switch dimension {
	case "user":
		return strconv.Itoa(e.log.UserId), e.log.Username
	case "model":
		return e.log.ModelName, e.log.ModelName
	case "day":
		day := time.Unix(e.log.CreatedAt, 0).In(location).Format("2006-01-02")
		return day, day
	case "token":
		return fmt.Sprintf("%d/%d", e.log.UserId, e.log.TokenId), e.log.TokenName
	case "group":
		return e.log.Group, e.log.Group
	case "channel":
		return strconv.Itoa(e.log.ChannelId), strconv.Itoa(e.log.ChannelId)
	}
	return "", ""
}

func buildBillSheets(ctx context.Context, logs []model.Log, options BillExportOptions, admin bool, quotaPerUnit float64) ([]billSheet, error) {
	if quotaPerUnit <= 0 || math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) {
		return nil, errors.New("额度换算单位无效，无法生成账单")
	}
	unit := decimal.NewFromFloat(quotaPerUnit)
	location, err := time.LoadLocation(options.Timezone)
	if err != nil {
		return nil, err
	}
	total := billTotals{}
	type groupTotal struct {
		label  string
		totals billTotals
	}
	groups := make(map[string]map[string]*groupTotal)
	for _, dimension := range options.Dimensions {
		groups[dimension] = make(map[string]*groupTotal)
	}
	for _, log := range logs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e := makeBillEntry(log)
		total.add(e)
		for dimension, rows := range groups {
			key, label := billDimension(e, dimension, location)
			if rows[key] == nil {
				rows[key] = &groupTotal{label: label}
			}
			rows[key].totals.add(e)
		}
	}
	sheets := []billSheet{{name: "账单概览", headers: []interface{}{"项目", "内容"}, rows: func(emit func([]interface{}) error) error {
		filters, _ := json.Marshal(options.BillExportFilter)
		rows := [][]interface{}{
			{"账单类型", "消费账单（非支付凭证、非完整资金流水）"},
			{"开始时间（含）", time.Unix(options.StartTimestamp, 0).In(location).Format(time.DateTime)},
			{"结束时间（含）", time.Unix(options.EndTimestamp, 0).In(location).Format(time.DateTime)},
			{"时区", options.Timezone}, {"生成时间", time.Now().In(location).Format(time.DateTime)},
			{"筛选条件", string(filters)}, {"日志类型", billTypeName(options.LogType)},
			{"记录数", total.entries}, {"消费记录数（含任务预扣和补扣，非去重请求数）", total.consumeEntries},
			{"额度换算单位（导出时配置）", quotaPerUnit},
			{"金额口径", "钱包折算金额 = 原始额度 / 导出时额度单位；非历史支付金额，不使用当前汇率换算其他币种"},
			{"订阅口径", "订阅消耗为额度用量，不是钱包扣款，也不重复计入套餐购买金额"},
			{"退款口径", "额度退回按退回发生时间入账；可能对应周期外消费，不代表支付退款"},
			{"历史数据", "计费来源缺失单列为未记录；不推断钱包扣费。仅包含仍保留的日志，不提供期初/期末余额"},
			{"统计范围", "API/任务消费、市场消耗、项目消耗、额度退回；不含充值、奖励、管理、系统和错误日志"},
			{"用量说明", "Token 用量按原日志记录，缓存用量可能包含在输入中，不额外相加；空白表示未记录"},
			{"精度说明", "原始额度为核对依据；钱包折算金额保留 10 位小数。超过 Excel 15 位有效数字的数值按文本保存"},
		}
		for i, value := range billAmountCells(total.amounts, unit) {
			rows = append(rows, []interface{}{billAmountHeaders[i], value})
		}
		for _, row := range rows {
			if err := emit(row); err != nil {
				return err
			}
		}
		return nil
	}}}
	for _, dimension := range options.Dimensions {
		rows := groups[dimension]
		keys := make([]string, 0, len(rows))
		for key := range rows {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		headers := append([]interface{}{"维度标识", "名称", "记录数", "消费记录数", "输入Token", "输出Token"}, billAmountHeaders...)
		sheets = append(sheets, billSheet{name: billDimensions[dimension], headers: headers, rows: func(emit func([]interface{}) error) error {
			for _, key := range keys {
				group := rows[key]
				b := group.totals
				row := []interface{}{key, group.label, b.entries, b.consumeEntries, billNumber(b.prompt), billNumber(b.completion)}
				if err := emit(append(row, billAmountCells(b.amounts, unit)...)); err != nil {
					return err
				}
			}
			return nil
		}})
	}
	headers := []interface{}{"发生时间", "记录编号", "请求ID", "任务ID", "用户ID", "用户名", "令牌ID", "令牌名称", "分组", "业务类型", "模型", "计费来源", "订阅ID", "套餐名称", "输入Token", "输出Token", "缓存读取Token", "缓存创建Token"}
	headers = append(headers, billAmountHeaders...)
	if admin {
		headers = append(headers, "渠道ID")
	}
	sheets = append(sheets, billSheet{name: "消费明细", headers: headers, rows: func(emit func([]interface{}) error) error {
		for _, log := range logs {
			e := makeBillEntry(log)
			cache, creation := interface{}(""), interface{}("")
			if e.meta.CacheTokens != nil {
				cache = billNumber(decimal.NewFromInt(*e.meta.CacheTokens))
			}
			if e.meta.CacheCreationTokens != nil {
				creation = billNumber(decimal.NewFromInt(*e.meta.CacheCreationTokens))
			}
			subscriptionID := ""
			if e.meta.SubscriptionID != 0 {
				subscriptionID = strconv.FormatInt(e.meta.SubscriptionID, 10)
			}
			row := []interface{}{time.Unix(log.CreatedAt, 0).In(location).Format(time.DateTime), strconv.Itoa(log.Id), log.RequestId, e.meta.TaskID, strconv.Itoa(log.UserId), log.Username, strconv.Itoa(log.TokenId), log.TokenName, log.Group, billTypeName(log.Type), log.ModelName, e.source, subscriptionID, e.meta.PlanTitle, billNumber(decimal.NewFromInt(int64(log.PromptTokens))), billNumber(decimal.NewFromInt(int64(log.CompletionTokens))), cache, creation}
			row = append(row, billAmountCells(e.amounts, unit)...)
			if admin {
				row = append(row, strconv.Itoa(log.ChannelId))
			}
			if err := emit(row); err != nil {
				return err
			}
		}
		return nil
	}})
	return sheets, nil
}

// CSVs opened in spreadsheet applications must treat user-controlled text as
// text, including formulas hidden behind leading whitespace.
func billCSVCell(value interface{}) string {
	if text, ok := value.(string); ok {
		trimmed := strings.TrimLeftFunc(text, unicode.IsSpace)
		if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
			// IDs and large exact values arrive as strings; retain their text
			// semantics when Excel opens a CSV (leading zeros, >15 digits).
			return "'" + text
		}
		if strings.ContainsAny(text, "\t\r\n") || (len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0]))) {
			return "'" + text
		}
		return text
	}
	return fmt.Sprint(value)
}

func WriteBillExport(ctx context.Context, writer io.Writer, logs []model.Log, options BillExportOptions, admin bool, quotaPerUnit float64) error {
	writer = billContextWriter{ctx: ctx, writer: writer}
	sheets, err := buildBillSheets(ctx, logs, options, admin, quotaPerUnit)
	if err != nil {
		return err
	}
	if options.Format == "csv" {
		archive := zip.NewWriter(writer)
		for _, sheet := range sheets {
			file, err := archive.Create(sheet.name + ".csv")
			if err != nil {
				_ = archive.Close()
				return err
			}
			if _, err := io.WriteString(file, "\xef\xbb\xbf"); err != nil {
				_ = archive.Close()
				return err
			}
			csvWriter := csv.NewWriter(file)
			csvWriter.UseCRLF = true
			emit := func(row []interface{}) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				cells := make([]string, len(row))
				for i, value := range row {
					cells[i] = billCSVCell(value)
				}
				return csvWriter.Write(cells)
			}
			err = emit(sheet.headers)
			if err == nil {
				err = sheet.rows(emit)
			}
			csvWriter.Flush()
			if err == nil {
				err = csvWriter.Error()
			}
			if err != nil {
				_ = archive.Close()
				return err
			}
		}
		return archive.Close()
	}
	book := excelize.NewFile()
	defer book.Close()
	headerStyle, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"2457A7"}}})
	if err != nil {
		return err
	}
	numberFormat := "0.##########"
	numberStyle, err := book.NewStyle(&excelize.Style{CustomNumFmt: &numberFormat})
	if err != nil {
		return err
	}
	for i, sheet := range sheets {
		if i == 0 {
			err = book.SetSheetName("Sheet1", sheet.name)
		} else {
			_, err = book.NewSheet(sheet.name)
		}
		if err != nil {
			return err
		}
		stream, err := book.NewStreamWriter(sheet.name)
		if err != nil {
			return err
		}
		if err := stream.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
			return err
		}
		if err := stream.SetColWidth(1, len(sheet.headers), 22); err != nil {
			return err
		}
		if i == 0 {
			if err := stream.SetColWidth(2, 2, 100); err != nil {
				return err
			}
		}
		header := make([]interface{}, len(sheet.headers))
		for j, value := range sheet.headers {
			header[j] = excelize.Cell{StyleID: headerStyle, Value: value}
		}
		if err := stream.SetRow("A1", header); err != nil {
			return err
		}
		rowNumber := 1
		err = sheet.rows(func(row []interface{}) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			rowNumber++
			for j, value := range row {
				if _, ok := value.(float64); ok {
					row[j] = excelize.Cell{StyleID: numberStyle, Value: value}
				}
			}
			return stream.SetRow(fmt.Sprintf("A%d", rowNumber), row)
		})
		if err != nil {
			return err
		}
		lastCell, err := excelize.CoordinatesToCellName(len(sheet.headers), rowNumber)
		if err != nil {
			return err
		}
		if err := book.AutoFilter(sheet.name, "A1:"+lastCell, nil); err != nil {
			return err
		}
		if err := stream.Flush(); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return book.Write(writer)
}

type billContextWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (w billContextWriter) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.writer.Write(data)
}
