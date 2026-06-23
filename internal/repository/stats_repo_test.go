package repository

import (
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Component{}, &models.StockLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedStatsFixtures(t *testing.T, db *gorm.DB) (uint, uint) {
	t.Helper()

	category := models.Category{Name: "电阻"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	compA := models.Component{
		CategoryID:     category.ID,
		Name:           "电阻A",
		StockQuantity:  10,
		UnitPriceMicro: 1000000, // 1元/件
	}
	compB := models.Component{
		CategoryID:     category.ID,
		Name:           "电阻B",
		StockQuantity:  5,
		UnitPriceMicro: 2000000, // 2元/件
	}
	if err := db.Create(&compA).Error; err != nil {
		t.Fatalf("create component A: %v", err)
	}
	if err := db.Create(&compB).Error; err != nil {
		t.Fatalf("create component B: %v", err)
	}

	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)

	logs := []models.StockLog{
		{
			ComponentID:     compA.ID,
			ChangeAmount:    10,
			TotalPriceCents: 1000,
			Reason:          "本月入库",
			CreatedAt:       now,
		},
		{
			ComponentID:     compA.ID,
			ChangeAmount:    -2,
			TotalPriceCents: 200,
			Reason:          "本月出库",
			CreatedAt:       now,
		},
		{
			ComponentID:     compB.ID,
			ChangeAmount:    20,
			TotalPriceCents: 3000,
			Reason:          "上月入库",
			CreatedAt:       lastMonth,
		},
		{
			ComponentID:     compA.ID,
			ChangeAmount:    0,
			TotalPriceCents: 500,
			Reason:          "补录价格",
			CreatedAt:       now,
		},
	}
	for i := range logs {
		if err := db.Create(&logs[i]).Error; err != nil {
			t.Fatalf("create stock log: %v", err)
		}
	}

	revokedAt := now
	revokedLog := models.StockLog{
		ComponentID:     compA.ID,
		ChangeAmount:    3,
		TotalPriceCents: 300,
		Reason:          "已撤销入库",
		RevokedAt:       &revokedAt,
		CreatedAt:       now,
	}
	if err := db.Create(&revokedLog).Error; err != nil {
		t.Fatalf("create revoked log: %v", err)
	}

	reversalOf := revokedLog.ID
	reversalLog := models.StockLog{
		ComponentID:     compA.ID,
		ChangeAmount:    -3,
		TotalPriceCents: 300,
		Reason:          "撤销冲销",
		ReversalOfID:    &reversalOf,
		CreatedAt:       now,
	}
	if err := db.Create(&reversalLog).Error; err != nil {
		t.Fatalf("create reversal log: %v", err)
	}

	return compA.ID, compB.ID
}

func TestStatsRepositoryGetDashboardStatsAll(t *testing.T) {
	db := setupStatsTestDB(t)
	seedStatsFixtures(t, db)

	repo := NewStatsRepository(db)
	stats, err := repo.GetDashboardStats(StatsRangeAll)
	if err != nil {
		t.Fatalf("GetDashboardStats: %v", err)
	}

	if stats.ComponentCount != 2 {
		t.Fatalf("component_count = %d, want 2", stats.ComponentCount)
	}
	if stats.CategoryCount != 1 {
		t.Fatalf("category_count = %d, want 1", stats.CategoryCount)
	}
	if stats.TotalStock != 15 {
		t.Fatalf("total_stock = %d, want 15", stats.TotalStock)
	}
	if stats.InventoryValueCents != 2000 {
		t.Fatalf("inventory_value_cents = %d, want 2000", stats.InventoryValueCents)
	}
	if stats.InboundQuantity != 30 {
		t.Fatalf("inbound_quantity = %d, want 30", stats.InboundQuantity)
	}
	if stats.OutboundQuantity != 2 {
		t.Fatalf("outbound_quantity = %d, want 2", stats.OutboundQuantity)
	}
	if stats.InboundCostCents != 4000 {
		t.Fatalf("inbound_cost_cents = %d, want 4000", stats.InboundCostCents)
	}
	if stats.RangeStart != nil {
		t.Fatalf("range_start should be nil for all")
	}
}

func TestStatsRepositoryGetDashboardStatsMonth(t *testing.T) {
	db := setupStatsTestDB(t)
	seedStatsFixtures(t, db)

	repo := NewStatsRepository(db)
	stats, err := repo.GetDashboardStats(StatsRangeMonth)
	if err != nil {
		t.Fatalf("GetDashboardStats: %v", err)
	}

	if stats.InboundQuantity != 10 {
		t.Fatalf("inbound_quantity = %d, want 10", stats.InboundQuantity)
	}
	if stats.OutboundQuantity != 2 {
		t.Fatalf("outbound_quantity = %d, want 2", stats.OutboundQuantity)
	}
	if stats.InboundCostCents != 1000 {
		t.Fatalf("inbound_cost_cents = %d, want 1000", stats.InboundCostCents)
	}
	if stats.RangeStart == nil || stats.RangeEnd == nil {
		t.Fatal("range_start and range_end should be set for month")
	}
}

func TestParseStatsRangeInvalid(t *testing.T) {
	_, _, err := parseStatsRange("invalid")
	if err != ErrInvalidStatsRange {
		t.Fatalf("expected ErrInvalidStatsRange, got %v", err)
	}
}

func TestParseStatsRangeQuarter(t *testing.T) {
	start, end, err := parseStatsRange(StatsRangeQuarter)
	if err != nil {
		t.Fatalf("parseStatsRange: %v", err)
	}
	if start == nil || end == nil {
		t.Fatal("quarter range should have start and end")
	}

	now := time.Now()
	expectedMonth := time.Month(((int(now.Month())-1)/3)*3 + 1)
	if start.Month() != expectedMonth || start.Day() != 1 {
		t.Fatalf("quarter start = %v, want first day of quarter", start)
	}
}
