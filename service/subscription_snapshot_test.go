package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSettleSubscriptionSnapshotRefundsOverdraftFirst(t *testing.T) {
	used, overdraft, remain := settleSubscriptionSnapshot(18000000, 18000000, 750000, -226161)

	assert.Equal(t, int64(18000000), used)
	assert.Equal(t, int64(523839), overdraft)
	assert.Equal(t, int64(0), remain)
}

func TestSettleSubscriptionSnapshotRefundsRemainAfterOverdraft(t *testing.T) {
	used, overdraft, remain := settleSubscriptionSnapshot(18000000, 18000000, 100000, -226161)

	assert.Equal(t, int64(17873839), used)
	assert.Equal(t, int64(0), overdraft)
	assert.Equal(t, int64(126161), remain)
}
