package price

// UnitPriceCents 将总价按数量整数分摊为单价（分），不能整除时向下取整。
func UnitPriceCents(totalPriceCents int64, quantity int) int64 {
	if quantity <= 0 || totalPriceCents <= 0 {
		return 0
	}
	return totalPriceCents / int64(quantity)
}

// TotalPriceCents 将单价（分）按数量计算总价（分）。
func TotalPriceCents(unitPriceCents int64, quantity int) int64 {
	if quantity <= 0 || unitPriceCents <= 0 {
		return 0
	}
	return unitPriceCents * int64(quantity)
}

// YuanToCents 将元（浮点）四舍五入换算为分。
func YuanToCents(yuan float64) int64 {
	if yuan <= 0 {
		return 0
	}
	return int64(yuan*100 + 0.5)
}

// WeightedAverageUnitPriceCents 按库存加权平均计算入库后的参考单价（分）。
// 无历史库存或历史单价时，直接使用本次入库分摊单价。
func WeightedAverageUnitPriceCents(oldQty int, oldUnitCents int64, inQty int, inTotalCents int64) int64 {
	if inQty <= 0 || inTotalCents <= 0 {
		return 0
	}
	if oldQty <= 0 || oldUnitCents <= 0 {
		return UnitPriceCents(inTotalCents, inQty)
	}
	newQty := oldQty + inQty
	if newQty <= 0 {
		return 0
	}
	totalValue := int64(oldQty)*oldUnitCents + inTotalCents
	return totalValue / int64(newQty)
}

// ReverseAverageUnitPriceCents 撤销入库时，按扣回本次贡献的价值反算回退后的参考单价（分）。
func ReverseAverageUnitPriceCents(curQty int, curUnitCents int64, inQty int, inTotalCents int64) int64 {
	if inQty <= 0 || inTotalCents <= 0 {
		return curUnitCents
	}
	remainQty := curQty - inQty
	if remainQty <= 0 {
		return 0
	}
	remainValue := int64(curQty)*curUnitCents - inTotalCents
	if remainValue <= 0 {
		return 0
	}
	return remainValue / int64(remainQty)
}
