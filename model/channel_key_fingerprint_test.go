package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetKeyByFingerprintSurvivesKeyReordering(t *testing.T) {
	channel := &Channel{Key: "key-a\nkey-b", ChannelInfo: ChannelInfo{IsMultiKey: true}}
	fingerprint := ChannelKeyFingerprint("key-b")

	key, index, found := channel.GetKeyByFingerprint(fingerprint)
	require.True(t, found)
	assert.Equal(t, "key-b", key)
	assert.Equal(t, 1, index)

	channel.Key = "key-b\nkey-a"
	key, index, found = channel.GetKeyByFingerprint(fingerprint)
	require.True(t, found)
	assert.Equal(t, "key-b", key)
	assert.Equal(t, 0, index)
}

func TestTaskUserTaskIDUniqueIndex(t *testing.T) {
	truncateTables(t)
	first := &Task{UserId: 7, TaskID: "cgt-same", Platform: "54"}
	second := &Task{UserId: 7, TaskID: "cgt-same", Platform: "54"}

	require.NoError(t, DB.Create(first).Error)
	assert.Error(t, DB.Create(second).Error)
	require.NoError(t, DB.Create(&Task{UserId: 8, TaskID: "cgt-same", Platform: "54"}).Error)
}

func TestRemoveDuplicateUserTasksBeforeUniqueIndexKeepsNewest(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Migrator().DropIndex(&Task{}, "idx_user_task"))
	t.Cleanup(func() {
		_ = DB.Migrator().CreateIndex(&Task{}, "idx_user_task")
	})

	first := &Task{UserId: 9, TaskID: "cgt-duplicate", Platform: "54"}
	second := &Task{UserId: 9, TaskID: "cgt-duplicate", Platform: "54"}
	require.NoError(t, DB.Create(first).Error)
	require.NoError(t, DB.Create(second).Error)
	require.NoError(t, removeDuplicateUserTasksBeforeUniqueIndex())

	var tasks []Task
	require.NoError(t, DB.Where("user_id = ? AND task_id = ?", 9, "cgt-duplicate").Find(&tasks).Error)
	require.Len(t, tasks, 1)
	assert.Equal(t, second.ID, tasks[0].ID)
}
