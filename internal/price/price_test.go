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

func TestWeightedAverageUnitPriceCents(t *testing.T) {
	tests := []struct {
		name         string
		oldQty       int
		oldUnitCents int64
		inQty        int
		inTotalCents int64
		want         int64
	}{
		{
			name:         "user example 10@10 + 1@12",
			oldQty:       10,
			oldUnitCents: 1000,
			inQty:        1,
			inTotalCents: 1200,
			want:         1018,
		},
		{
			name:         "no history qty uses inbound unit price",
			oldQty:       0,
			oldUnitCents: 0,
			inQty:        5,
			inTotalCents: 550,
			want:         110,
		},
		{
			name:         "no history unit price uses inbound unit price",
			oldQty:       10,
			oldUnitCents: 0,
			inQty:        2,
			inTotalCents: 240,
			want:         120,
		},
		{
			name:         "zero inbound qty",
			oldQty:       10,
			oldUnitCents: 1000,
			inQty:        0,
			inTotalCents: 1200,
			want:         0,
		},
		{
			name:         "zero inbound total",
			oldQty:       10,
			oldUnitCents: 1000,
			inQty:        1,
			inTotalCents: 0,
			want:         0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WeightedAverageUnitPriceCents(tt.oldQty, tt.oldUnitCents, tt.inQty, tt.inTotalCents)
			if got != tt.want {
				t.Errorf("WeightedAverageUnitPriceCents(%d, %d, %d, %d) = %d, want %d",
					tt.oldQty, tt.oldUnitCents, tt.inQty, tt.inTotalCents, got, tt.want)
			}
		})
	}
}

func TestReverseAverageUnitPriceCents(t *testing.T) {
	tests := []struct {
		name         string
		curQty       int
		curUnitCents int64
		inQty        int
		inTotalCents int64
		want         int64
	}{
		{
			name:         "reverse user example with rounding drift",
			curQty:       11,
			curUnitCents: 1018,
			inQty:        1,
			inTotalCents: 1200,
			want:         999,
		},
		{
			name:         "round trip exact",
			curQty:       20,
			curUnitCents: 1100,
			inQty:        10,
			inTotalCents: 12000,
			want:         1000,
		},
		{
			name:         "stock cleared after revoke",
			curQty:       1,
			curUnitCents: 1200,
			inQty:        1,
			inTotalCents: 1200,
			want:         0,
		},
		{
			name:         "zero inbound qty keeps current unit price",
			curQty:       11,
			curUnitCents: 1010,
			inQty:        0,
			inTotalCents: 1200,
			want:         1010,
		},
		{
			name:         "zero inbound total keeps current unit price",
			curQty:       11,
			curUnitCents: 1010,
			inQty:        1,
			inTotalCents: 0,
			want:         1010,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReverseAverageUnitPriceCents(tt.curQty, tt.curUnitCents, tt.inQty, tt.inTotalCents)
			if got != tt.want {
				t.Errorf("ReverseAverageUnitPriceCents(%d, %d, %d, %d) = %d, want %d",
					tt.curQty, tt.curUnitCents, tt.inQty, tt.inTotalCents, got, tt.want)
			}
		})
	}
}
