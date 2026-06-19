package price

import "testing"

func TestUnitPriceCents(t *testing.T) {
	tests := []struct {
		total    int64
		quantity int
		want     int64
	}{
		{1000, 10, 100},
		{1005, 10, 100},
		{0, 10, 0},
		{100, 0, 0},
	}
	for _, tt := range tests {
		got := UnitPriceCents(tt.total, tt.quantity)
		if got != tt.want {
			t.Errorf("UnitPriceCents(%d, %d) = %d, want %d", tt.total, tt.quantity, got, tt.want)
		}
	}
}

func TestTotalPriceCents(t *testing.T) {
	tests := []struct {
		unitPrice int64
		quantity  int
		want      int64
	}{
		{100, 10, 1000},
		{0, 10, 0},
		{100, 0, 0},
		{-50, 5, 0},
	}
	for _, tt := range tests {
		got := TotalPriceCents(tt.unitPrice, tt.quantity)
		if got != tt.want {
			t.Errorf("TotalPriceCents(%d, %d) = %d, want %d", tt.unitPrice, tt.quantity, got, tt.want)
		}
	}
}

func TestYuanToCents(t *testing.T) {
	if got := YuanToCents(12.34); got != 1234 {
		t.Errorf("YuanToCents(12.34) = %d, want 1234", got)
	}
	if got := YuanToCents(0); got != 0 {
		t.Errorf("YuanToCents(0) = %d, want 0", got)
	}
}
