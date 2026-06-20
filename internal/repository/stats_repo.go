package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrInvalidStatsRange = errors.New("无效的统计时间范围")

const (
	StatsRangeMonth   = "month"
	StatsRangeQuarter = "quarter"
	StatsRangeAll     = "all"
)

// DashboardStats 仪表盘聚合统计
type DashboardStats struct {
	Range               string     `json:"range"`
	RangeStart          *time.Time `json:"range_start"`
	RangeEnd            *time.Time `json:"range_end"`
	ComponentCount      int64      `json:"component_count"`
	CategoryCount       int64      `json:"category_count"`
	TotalStock          int64      `json:"total_stock"`
	InventoryValueCents int64      `json:"inventory_value_cents"`
	InboundQuantity     int64      `json:"inbound_quantity"`
	OutboundQuantity    int64      `json:"outbound_quantity"`
	InboundCostCents    int64      `json:"inbound_cost_cents"`
}

type StatsRepository struct {
	db *gorm.DB
}

func NewStatsRepository(db *gorm.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

func (r *StatsRepository) GetDashboardStats(rangeKey string) (*DashboardStats, error) {
	rangeStart, rangeEnd, err := parseStatsRange(rangeKey)
	if err != nil {
		return nil, err
	}

	stats := &DashboardStats{
		Range:      rangeKey,
		RangeStart: rangeStart,
		RangeEnd:   rangeEnd,
	}

	if err := r.db.Table("components").Count(&stats.ComponentCount).Error; err != nil {
		return nil, err
	}

	if err := r.db.Table("categories").Count(&stats.CategoryCount).Error; err != nil {
		return nil, err
	}

	type sumResult struct {
		Total int64
	}
	var stockSum sumResult
	if err := r.db.Table("components").
		Select("COALESCE(SUM(stock_quantity), 0) AS total").
		Scan(&stockSum).Error; err != nil {
		return nil, err
	}
	stats.TotalStock = stockSum.Total

	var valueSum sumResult
	if err := r.db.Table("components").
		Select("COALESCE(SUM(stock_quantity * unit_price_cents), 0) AS total").
		Where("stock_quantity > 0 AND unit_price_cents > 0").
		Scan(&valueSum).Error; err != nil {
		return nil, err
	}
	stats.InventoryValueCents = valueSum.Total

	logQuery := r.db.Table("stock_logs").
		Where("revoked_at IS NULL AND reversal_of_id IS NULL AND change_amount != 0")
	if rangeStart != nil {
		logQuery = logQuery.Where("created_at >= ? AND created_at <= ?", *rangeStart, *rangeEnd)
	}

	var inboundQty sumResult
	if err := logQuery.Session(&gorm.Session{}).
		Where("change_amount > 0").
		Select("COALESCE(SUM(change_amount), 0) AS total").
		Scan(&inboundQty).Error; err != nil {
		return nil, err
	}
	stats.InboundQuantity = inboundQty.Total

	var outboundQty sumResult
	if err := logQuery.Session(&gorm.Session{}).
		Where("change_amount < 0").
		Select("COALESCE(SUM(ABS(change_amount)), 0) AS total").
		Scan(&outboundQty).Error; err != nil {
		return nil, err
	}
	stats.OutboundQuantity = outboundQty.Total

	var inboundCost sumResult
	if err := logQuery.Session(&gorm.Session{}).
		Where("change_amount > 0 AND total_price_cents > 0").
		Select("COALESCE(SUM(total_price_cents), 0) AS total").
		Scan(&inboundCost).Error; err != nil {
		return nil, err
	}
	stats.InboundCostCents = inboundCost.Total

	return stats, nil
}

func parseStatsRange(rangeKey string) (*time.Time, *time.Time, error) {
	now := time.Now()
	end := now

	switch rangeKey {
	case StatsRangeAll:
		return nil, &end, nil
	case StatsRangeMonth:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return &start, &end, nil
	case StatsRangeQuarter:
		quarterMonth := time.Month(((int(now.Month())-1)/3)*3 + 1)
		start := time.Date(now.Year(), quarterMonth, 1, 0, 0, 0, 0, now.Location())
		return &start, &end, nil
	default:
		return nil, nil, ErrInvalidStatsRange
	}
}
