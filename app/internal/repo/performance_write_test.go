package repo

import "testing"

func TestCommissionForIndexBoundaries(t *testing.T) {
	p := func(n int) *int { return &n }
	tiers := []CommissionTier{{From: 1, To: p(3), Amount: 10}, {From: 4, To: p(5), Amount: 20}, {From: 7, To: nil, Amount: 30}}
	for _, tc := range []struct{ index, want int }{{-1, 0}, {0, 0}, {1, 10}, {3, 10}, {4, 20}, {5, 20}, {6, 0}, {7, 30}, {1000000, 30}} {
		t.Run(itoa(tc.index+2), func(t *testing.T) {
			if got := CommissionForIndex(tiers, tc.index); got != tc.want {
				t.Fatalf("index %d: got %d, want %d", tc.index, got, tc.want)
			}
		})
	}
	if CommissionForIndex(nil, 1) != 0 {
		t.Fatal("nil rules must yield zero")
	}
	if CommissionForIndex([]CommissionTier{{From: 1, To: p(4), Amount: 10}, {From: 2, Amount: 20}}, 2) != 10 {
		t.Fatal("the first matching tier wins")
	}
}

func TestValidMoney(t *testing.T) {
	for _, tc := range []struct {
		amount float64
		want   bool
	}{{0, false}, {-1, false}, {0.001, false}, {0.01, true}, {12.34, true}, {1e11, false}} {
		if ValidMoney(tc.amount) != tc.want {
			t.Errorf("amount %v", tc.amount)
		}
	}
}
