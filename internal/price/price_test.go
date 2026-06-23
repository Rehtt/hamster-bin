package price

import "testing"

func TestUnitPriceMicro(t *testing.T) {
	tests := []struct {
		total    int64
		quantity int
		want     int64
	}{
		{1000, 10, 1000000}, // 10元/10件 = 1元/件 = 1,000,000 微元
		{1005, 10, 1005000}, // 10.05元/10件
		{50, 100, 5000},     // 0.5元/100件 = 0.005元/件 = 5000 微元
		{0, 10, 0},
		{100, 0, 0},
	}
	for _, tt := range tests {
		got := UnitPriceMicro(tt.total, tt.quantity)
		if got != tt.want {
			t.Errorf("UnitPriceMicro(%d, %d) = %d, want %d", tt.total, tt.quantity, got, tt.want)
		}
	}
}

func TestOutboundTotalCents(t *testing.T) {
	tests := []struct {
		unitMicro int64
		quantity  int
		want      int64
	}{
		{1000000, 10, 1000}, // 1元/件 × 10 = 10元 = 1000分
		{5000, 100, 50},     // 0.005元/件 × 100 = 0.5元 = 50分
		{0, 10, 0},
		{1000000, 0, 0},
		{-5000, 5, 0},
	}
	for _, tt := range tests {
		got := OutboundTotalCents(tt.unitMicro, tt.quantity)
		if got != tt.want {
			t.Errorf("OutboundTotalCents(%d, %d) = %d, want %d", tt.unitMicro, tt.quantity, got, tt.want)
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

func TestWeightedAverageUnitPriceMicro(t *testing.T) {
	tests := []struct {
		name         string
		oldQty       int
		oldUnitMicro int64
		inQty        int
		inTotalCents int64
		want         int64
	}{
		{
			name:         "user example 10@10 + 1@12",
			oldQty:       10,
			oldUnitMicro: 10000000, // 10元/件 = 1000分
			inQty:        1,
			inTotalCents: 1200, // 12元
			want:         10181818,
		},
		{
			name:         "no history qty uses inbound unit price",
			oldQty:       0,
			oldUnitMicro: 0,
			inQty:        5,
			inTotalCents: 550,
			want:         1100000,
		},
		{
			name:         "no history unit price uses inbound unit price",
			oldQty:       10,
			oldUnitMicro: 0,
			inQty:        2,
			inTotalCents: 240,
			want:         1200000,
		},
		{
			name:         "zero inbound qty",
			oldQty:       10,
			oldUnitMicro: 1000000,
			inQty:        0,
			inTotalCents: 1200,
			want:         0,
		},
		{
			name:         "zero inbound total",
			oldQty:       10,
			oldUnitMicro: 1000000,
			inQty:        1,
			inTotalCents: 0,
			want:         0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WeightedAverageUnitPriceMicro(tt.oldQty, tt.oldUnitMicro, tt.inQty, tt.inTotalCents)
			if got != tt.want {
				t.Errorf("WeightedAverageUnitPriceMicro(%d, %d, %d, %d) = %d, want %d",
					tt.oldQty, tt.oldUnitMicro, tt.inQty, tt.inTotalCents, got, tt.want)
			}
		})
	}
}

func TestReverseAverageUnitPriceMicro(t *testing.T) {
	tests := []struct {
		name         string
		curQty       int
		curUnitMicro int64
		inQty        int
		inTotalCents int64
		want         int64
	}{
		{
			name:         "reverse user example with rounding drift",
			curQty:       11,
			curUnitMicro: 10181818,
			inQty:        1,
			inTotalCents: 1200,
			want:         9999999,
		},
		{
			name:         "round trip exact",
			curQty:       20,
			curUnitMicro: 11000000, // 11元/件
			inQty:        10,
			inTotalCents: 12000,    // 120元
			want:         10000000, // 10元/件
		},
		{
			name:         "stock cleared after revoke",
			curQty:       1,
			curUnitMicro: 12000000, // 12元/件
			inQty:        1,
			inTotalCents: 1200,
			want:         0,
		},
		{
			name:         "zero inbound qty keeps current unit price",
			curQty:       11,
			curUnitMicro: 1010000,
			inQty:        0,
			inTotalCents: 1200,
			want:         1010000,
		},
		{
			name:         "zero inbound total keeps current unit price",
			curQty:       11,
			curUnitMicro: 1010000,
			inQty:        1,
			inTotalCents: 0,
			want:         1010000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReverseAverageUnitPriceMicro(tt.curQty, tt.curUnitMicro, tt.inQty, tt.inTotalCents)
			if got != tt.want {
				t.Errorf("ReverseAverageUnitPriceMicro(%d, %d, %d, %d) = %d, want %d",
					tt.curQty, tt.curUnitMicro, tt.inQty, tt.inTotalCents, got, tt.want)
			}
		})
	}
}
