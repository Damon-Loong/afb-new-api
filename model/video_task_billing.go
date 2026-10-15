package model

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// CompleteVideoTask commits the terminal state and monetary delta together.
// Repeated polling/callbacks cannot charge twice, and DB failures leave the
// task unfinished so polling can retry. Deleted tokens do not prevent refunds.
func CompleteVideoTask(task *Task, from TaskStatus, actualQuota int) (bool, error) {
	if actualQuota < 0 || from == TaskStatusSuccess || from == TaskStatusFailure {
		return false, fmt.Errorf("invalid video settlement")
	}
	reserved := task.Quota
	delta := actualQuota - reserved
	updated := *task
	updated.Quota = actualQuota
	won := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&updated).Where("status = ?", from).Select("*").Updates(&updated)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if delta != 0 {
			if task.PrivateData.BillingSource == "subscription" && task.PrivateData.SubscriptionId > 0 {
				var sub UserSubscription
				if err := lockForUpdate(tx).First(&sub, task.PrivateData.SubscriptionId).Error; err != nil {
					return err
				}
				if err := applySubscriptionQuotaDeltaTx(tx, &sub, int64(delta)); err != nil {
					return err
				}
			} else {
				res := tx.Model(&User{}).Where("id = ?", task.UserId).UpdateColumn("quota", gorm.Expr("quota - ?", delta))
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected != 1 {
					return fmt.Errorf("video billing user not found")
				}
			}
			if task.PrivateData.TokenId > 0 {
				if err := tx.Model(&Token{}).Where("id = ? AND user_id = ?", task.PrivateData.TokenId, task.UserId).Updates(map[string]any{
					"remain_quota": gorm.Expr("remain_quota - ?", delta), "used_quota": gorm.Expr("used_quota + ?", delta),
				}).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&User{}).Where("id = ?", task.UserId).UpdateColumn("used_quota", gorm.Expr("used_quota + ?", delta)).Error; err != nil {
				return err
			}
			if err := tx.Model(&Channel{}).Where("id = ?", task.ChannelId).UpdateColumn("used_quota", gorm.Expr("used_quota + ?", delta)).Error; err != nil {
				return err
			}
		}
		won = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if won {
		task.Quota = actualQuota
		if err := InvalidateUserCache(task.UserId); err != nil {
			common.SysError("video billing user cache: " + err.Error())
		}
		if err := InvalidateUserTokensCache(task.UserId); err != nil {
			common.SysError("video billing token cache: " + err.Error())
		}
	}
	return won, nil
}
