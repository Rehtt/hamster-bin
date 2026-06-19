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
