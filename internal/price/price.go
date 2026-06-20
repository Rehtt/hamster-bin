package price

// MicroPerCent 1 分 = 10,000 微元（1 元 = 1,000,000 微元 = 100 分）
const MicroPerCent = 10000

// UnitPriceMicro 将总价（分）按数量分摊为单价（微元），四舍五入。
func UnitPriceMicro(totalPriceCents int64, quantity int) int64 {
	if quantity <= 0 || totalPriceCents <= 0 {
		return 0
	}
	return (totalPriceCents*MicroPerCent + int64(quantity)/2) / int64(quantity)
}

// OutboundTotalCents 将单价（微元）按数量计算出库成本总价（分），四舍五入。
func OutboundTotalCents(unitPriceMicro int64, quantity int) int64 {
	if quantity <= 0 || unitPriceMicro <= 0 {
		return 0
	}
	return (unitPriceMicro*int64(quantity) + MicroPerCent/2) / MicroPerCent
}

// YuanToCents 将元（浮点）四舍五入换算为分。
func YuanToCents(yuan float64) int64 {
	if yuan <= 0 {
		return 0
	}
	return int64(yuan*100 + 0.5)
}

// WeightedAverageUnitPriceMicro 按库存加权平均计算入库后的参考单价（微元）。
// 无历史库存或历史单价时，直接使用本次入库分摊单价。
func WeightedAverageUnitPriceMicro(oldQty int, oldUnitMicro int64, inQty int, inTotalCents int64) int64 {
	if inQty <= 0 || inTotalCents <= 0 {
		return 0
	}
	if oldQty <= 0 || oldUnitMicro <= 0 {
		return UnitPriceMicro(inTotalCents, inQty)
	}
	newQty := oldQty + inQty
	if newQty <= 0 {
		return 0
	}
	totalValue := int64(oldQty)*oldUnitMicro + inTotalCents*MicroPerCent
	return totalValue / int64(newQty)
}

// ReverseAverageUnitPriceMicro 撤销入库时，按扣回本次贡献的价值反算回退后的参考单价（微元）。
func ReverseAverageUnitPriceMicro(curQty int, curUnitMicro int64, inQty int, inTotalCents int64) int64 {
	if inQty <= 0 || inTotalCents <= 0 {
		return curUnitMicro
	}
	remainQty := curQty - inQty
	if remainQty <= 0 {
		return 0
	}
	remainValue := int64(curQty)*curUnitMicro - inTotalCents*MicroPerCent
	if remainValue <= 0 {
		return 0
	}
	return remainValue / int64(remainQty)
}
