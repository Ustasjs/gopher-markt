package money

import "testing"

func TestFromKopecks(t *testing.T) {
	tests := []struct {
		kopecks  int64
		expected float64
	}{
		{0, 0},
		{100, 1.0},
		{50050, 500.50},
		{1, 0.01},
		{75100, 751.0},
	}

	for _, tt := range tests {
		got := FromKopecks(tt.kopecks)
		if got != tt.expected {
			t.Errorf("FromKopecks(%d) = %v, want %v", tt.kopecks, got, tt.expected)
		}
	}
}

func TestToKopecks(t *testing.T) {
	tests := []struct {
		rubles   float64
		expected int64
	}{
		{0, 0},
		{1.0, 100},
		{500.50, 50050},
		{0.01, 1},
		{751.0, 75100},
		{0.005, 1},
		{0.004, 0},
	}

	for _, tt := range tests {
		got := ToKopecks(tt.rubles)
		if got != tt.expected {
			t.Errorf("ToKopecks(%v) = %d, want %d", tt.rubles, got, tt.expected)
		}
	}
}
