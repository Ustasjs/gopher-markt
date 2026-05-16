package money

import "math"

// FromKopecks converts kopecks (int64) to rubles (float64).
func FromKopecks(kopecks int64) float64 {
	return float64(kopecks) / 100
}

// ToKopecks converts rubles (float64) to kopecks (int64).
func ToKopecks(rubles float64) int64 {
	return int64(math.Round(rubles * 100))
}
