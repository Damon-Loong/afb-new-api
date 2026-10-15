package service

import (
	"context"
	"fmt"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func completeVideoBilling(ctx context.Context, task *model.Task, from model.TaskStatus, result *relaycommon.TaskInfo) error {
	snapshot := task.PrivateData.BillingContext.VideoBilling
	actual := 0
	var err error
	if task.Status == model.TaskStatusSuccess {
		// Missing usage/resolution is not a free call or a reason to silently
		// keep the reserve. Leave DB state unfinished and retry provider query.
		actual, err = snapshot.Quota(result.CompletionTokens, result.Resolution)
		if err != nil {
			task.Status = from
			task.FinishTime = 0
			task.Progress = "99%"
			task.Properties.BillingStatus = "pending"
			task.Properties.BillingMessage = err.Error()
			if _, saveErr := task.UpdateWithStatus(from); saveErr != nil {
				return saveErr
			}
			return fmt.Errorf("视频任务 %s 待核对计费: %w", task.TaskID, err)
		}
	}
	task.Properties.BillingStatus = "settled"
	task.Properties.BillingMessage = ""
	reserved := task.Quota
	won, err := model.CompleteVideoTask(task, from, actual)
	if err != nil || !won {
		return err
	}
	delta := actual - reserved
	logType, logQuota := model.LogTypeConsume, delta
	if delta < 0 {
		logType, logQuota = model.LogTypeRefund, -delta
	}
	other := taskBillingOther(task)
	other["billing_mode"] = "video_tokens"
	other["video_pricing_snapshot"] = snapshot
	other["completion_tokens"] = result.CompletionTokens
	other["resolution"] = result.Resolution
	other["duration_seconds"] = result.Duration
	other["task_id"], other["pre_consumed_quota"], other["actual_quota"] = task.TaskID, reserved, actual
	content := fmt.Sprintf("视频结算：%s，%.2f 秒，%d video tokens", result.Resolution, result.Duration, result.CompletionTokens)
	if task.Status == model.TaskStatusFailure {
		content = "视频任务失败/取消，退还预扣额度：" + task.FailReason
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId: task.UserId, LogType: logType, Quota: logQuota, Content: content,
		ChannelId: task.ChannelId, ModelName: taskModelName(task), TokenId: task.PrivateData.TokenId, Group: task.Group, Other: other,
	})
	return nil
}
