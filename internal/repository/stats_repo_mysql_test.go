package repository

import (
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestStatsRepositoryMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not set; skipping MySQL integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	var databaseName string
	if err := db.Raw("SELECT DATABASE()").Scan(&databaseName).Error; err != nil {
		t.Fatalf("get database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_test") {
		t.Fatal("TEST_MYSQL_DSN must select an isolated database whose name ends with _test")
	}

	// MySQL 临时表属于连接；固定连接，且只创建临时表，不执行应用迁移。
	err = db.Connection(func(conn *gorm.DB) error {
		for _, ddl := range []string{
			"CREATE TEMPORARY TABLE categories (id BIGINT PRIMARY KEY)",
			"CREATE TEMPORARY TABLE components (id BIGINT PRIMARY KEY, stock_quantity BIGINT, unit_price_micro BIGINT)",
			"CREATE TEMPORARY TABLE stock_logs (change_amount BIGINT, total_price_cents BIGINT, revoked_at DATETIME(6) NULL, reversal_of_id BIGINT NULL, created_at DATETIME(6))",
		} {
			if err := conn.Exec(ddl).Error; err != nil {
				return err
			}
		}

		repo := NewStatsRepository(conn)
		for _, rangeKey := range []string{StatsRangeMonth, StatsRangeQuarter, StatsRangeAll} {
			t.Run("empty/"+rangeKey, func(t *testing.T) {
				stats, err := repo.GetDashboardStats(rangeKey)
				if err != nil {
					t.Fatalf("GetDashboardStats: %v", err)
				}
				if stats.ComponentCount != 0 || stats.CategoryCount != 0 || stats.TotalStock != 0 ||
					stats.InventoryValueCents != 0 || stats.InboundQuantity != 0 || stats.OutboundQuantity != 0 || stats.InboundCostCents != 0 {
					t.Fatalf("expected empty inventory statistics, got %+v", stats)
				}
			})
		}

		if err := conn.Exec("INSERT INTO categories (id) VALUES (1)").Error; err != nil {
			return err
		}
		if err := conn.Exec("INSERT INTO components (id, stock_quantity, unit_price_micro) VALUES (1, 3, 333333), (2, 1, 6001)").Error; err != nil {
			return err
		}
		now := time.Now()
		thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastYear := thisMonth.AddDate(-1, 0, 0)
		if err := conn.Exec(`INSERT INTO stock_logs
			(change_amount, total_price_cents, revoked_at, reversal_of_id, created_at) VALUES
			(3, 100, NULL, NULL, ?), (-2, 67, NULL, NULL, ?), (5, 500, NULL, NULL, ?),
			(4, 400, ?, NULL, ?), (-4, 400, NULL, 1, ?), (0, 900, NULL, NULL, ?)`,
			thisMonth, thisMonth, lastYear, thisMonth, thisMonth, thisMonth, thisMonth).Error; err != nil {
			return err
		}
		for _, rangeKey := range []string{StatsRangeMonth, StatsRangeQuarter, StatsRangeAll} {
			t.Run("populated/"+rangeKey, func(t *testing.T) {
				stats, err := repo.GetDashboardStats(rangeKey)
				if err != nil {
					t.Fatalf("GetDashboardStats: %v", err)
				}
				wantInbound, wantCost := int64(3), int64(100)
				if rangeKey == StatsRangeAll {
					wantInbound, wantCost = 8, 600
				}
				if stats.ComponentCount != 2 || stats.CategoryCount != 1 || stats.TotalStock != 4 ||
					stats.InventoryValueCents != 100 || stats.InboundQuantity != wantInbound ||
					stats.OutboundQuantity != 2 || stats.InboundCostCents != wantCost {
					t.Fatalf("unexpected statistics: %+v (want inbound_quantity=%d, inbound_cost_cents=%d)", stats, wantInbound, wantCost)
				}
				if stats.Range != rangeKey || stats.RangeEnd == nil || (stats.RangeStart == nil) != (rangeKey == StatsRangeAll) {
					t.Fatalf("unexpected range metadata: %+v", stats)
				}
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("MySQL integration: %v", err)
	}
}
