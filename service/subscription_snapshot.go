package service

func settleSubscriptionSnapshot(amountTotal, amountUsedAfterPreConsume, overdraftAfterPreConsume, postDelta int64) (usedFinal, overdraftFinal, remainFinal int64) {
	usedFinal = amountUsedAfterPreConsume
	overdraftFinal = overdraftAfterPreConsume
	if postDelta == 0 {
		return normalizeSubscriptionSnapshot(amountTotal, usedFinal, overdraftFinal)
	}
	if amountTotal <= 0 {
		usedFinal += postDelta
		if usedFinal < 0 {
			usedFinal = 0
		}
		return normalizeSubscriptionSnapshot(amountTotal, usedFinal, overdraftFinal)
	}
	if postDelta > 0 {
		remain := amountTotal - usedFinal
		if remain < 0 {
			remain = 0
		}
		consumeFromRemain := postDelta
		if consumeFromRemain > remain {
			consumeFromRemain = remain
		}
		usedFinal += consumeFromRemain
		if usedFinal > amountTotal {
			usedFinal = amountTotal
		}
		if overdraft := postDelta - consumeFromRemain; overdraft > 0 {
			overdraftFinal += overdraft
		}
		return normalizeSubscriptionSnapshot(amountTotal, usedFinal, overdraftFinal)
	}

	refund := -postDelta
	if overdraftFinal > 0 {
		refundOverdraft := refund
		if refundOverdraft > overdraftFinal {
			refundOverdraft = overdraftFinal
		}
		overdraftFinal -= refundOverdraft
		refund -= refundOverdraft
	}
	if refund > 0 {
		if refund >= usedFinal {
			usedFinal = 0
		} else {
			usedFinal -= refund
		}
	}
	return normalizeSubscriptionSnapshot(amountTotal, usedFinal, overdraftFinal)
}

func normalizeSubscriptionSnapshot(amountTotal, used, overdraft int64) (int64, int64, int64) {
	if used < 0 {
		used = 0
	}
	if overdraft < 0 {
		overdraft = 0
	}
	if amountTotal > 0 && used > amountTotal {
		used = amountTotal
	}
	remain := int64(0)
	if amountTotal > 0 {
		remain = amountTotal - used
		if remain < 0 {
			remain = 0
		}
	}
	return used, overdraft, remain
}
