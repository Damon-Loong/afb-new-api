package service

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
)

const billExportLifetime = 30 * time.Minute

type BillExportJob struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Rows      int    `json:"rows"`
	Filename  string `json:"filename"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	ownerID   int
	admin     bool
	path      string
}

type billExportManager struct {
	mu      sync.Mutex
	jobs    map[string]*BillExportJob
	running int
}

var billExports = billExportManager{jobs: make(map[string]*BillExportJob)}

func StartBillExport(ownerID int, admin bool, options BillExportOptions) (BillExportJob, error) {
	if ownerID <= 0 {
		return BillExportJob{}, errors.New("无权导出账单")
	}
	if err := options.Validate(admin); err != nil {
		return BillExportJob{}, err
	}
	billExports.mu.Lock()
	defer billExports.mu.Unlock()
	owned := 0
	for _, job := range billExports.jobs {
		if job.ownerID == ownerID {
			owned++
			if job.Status == "running" {
				return BillExportJob{}, errors.New("已有账单正在生成，请稍后再试")
			}
		}
	}
	if billExports.running >= 2 || len(billExports.jobs) >= 32 || owned >= 5 {
		return BillExportJob{}, errors.New("账单导出任务较多，请等待已有任务过期后重试")
	}
	extension := ".xlsx"
	if options.Format == "csv" {
		extension = ".zip"
	}
	location, _ := time.LoadLocation(options.Timezone)
	job := &BillExportJob{
		ID: uuid.NewString(), Status: "running", ownerID: ownerID, admin: admin,
		Filename: "消费账单_" + time.Unix(options.StartTimestamp, 0).In(location).Format("2006-01-02") + "_" + time.Unix(options.EndTimestamp, 0).In(location).Format("2006-01-02") + extension,
	}
	billExports.jobs[job.ID] = job
	billExports.running++
	// Snapshot the conversion configuration before starting the background job.
	quotaPerUnit := common.QuotaPerUnit
	go billExports.generate(job.ID, options, quotaPerUnit)
	return *job, nil
}

func (m *billExportManager) generate(id string, options BillExportOptions, quotaPerUnit float64) {
	m.mu.Lock()
	job := *m.jobs[id]
	m.mu.Unlock()
	var path string
	var rows int
	var resultErr error
	defer func() {
		if recover() != nil {
			resultErr = errors.New("账单生成失败，请稍后重试")
			common.SysError("bill export worker panicked")
		}
		if resultErr != nil && path != "" {
			_ = os.Remove(path)
			path = ""
		}
		m.mu.Lock()
		entry := m.jobs[id]
		entry.Rows, entry.path = rows, path
		entry.Status = "ready"
		if resultErr != nil {
			entry.Status, entry.Message = "failed", resultErr.Error()
		}
		entry.ExpiresAt = time.Now().Add(billExportLifetime).Unix()
		m.running--
		m.mu.Unlock()
		time.AfterFunc(billExportLifetime, func() {
			m.mu.Lock()
			delete(m.jobs, id)
			m.mu.Unlock()
			if path != "" {
				_ = os.Remove(path)
			}
		})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	logs, err := model.GetBillExportLogs(ctx, options.BillExportFilter, job.ownerID, job.admin)
	if err != nil {
		common.SysError("bill export query failed: " + err.Error())
		if errors.Is(err, model.ErrBillExportTooLarge) {
			resultErr = err
		} else {
			resultErr = errors.New("账单查询失败，请检查筛选条件或稍后重试")
		}
		return
	}
	rows = len(logs)
	if rows == 0 {
		resultErr = errors.New("当前筛选条件下没有消费或额度退回记录")
		return
	}
	file, err := os.CreateTemp("", "new-api-bill-*")
	if err != nil {
		resultErr = errors.New("无法创建账单文件，请稍后重试")
		return
	}
	path = file.Name()
	defer file.Close()
	if err := WriteBillExport(ctx, file, logs, options, job.admin, quotaPerUnit); err != nil {
		common.SysError("bill export generation failed: " + err.Error())
		resultErr = errors.New("账单生成失败或超时，请缩小范围后重试")
		return
	}
	if err := file.Close(); err != nil {
		resultErr = errors.New("账单文件保存失败，请稍后重试")
	}
}

// Job ownership is checked independently of route authentication. An admin
// cannot download another admin's export by knowing its ID.
func GetBillExportJob(id string, ownerID int, admin bool) (BillExportJob, bool) {
	billExports.mu.Lock()
	defer billExports.mu.Unlock()
	job, ok := billExports.jobs[id]
	if !ok || job.ownerID != ownerID || job.admin != admin || (job.ExpiresAt > 0 && time.Now().Unix() >= job.ExpiresAt) {
		return BillExportJob{}, false
	}
	return *job, true
}

func OpenBillExport(id string, ownerID int, admin bool) (*os.File, BillExportJob, error) {
	job, ok := GetBillExportJob(id, ownerID, admin)
	if !ok || job.Status != "ready" {
		return nil, BillExportJob{}, errors.New("账单尚未完成、已过期或无权访问")
	}
	file, err := os.Open(job.path)
	return file, job, err
}
