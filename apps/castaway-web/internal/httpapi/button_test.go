package httpapi

import (
	"reflect"
	"testing"
)

func TestButtonAwards(t *testing.T) {
	for name, tc := range map[string]struct {
		presses map[string]int64
		want    map[string]int
	}{
		"ranks":           {map[string]int64{"a": 50, "b": 20, "c": 9, "d": 3}, map[string]int{"a": 2, "b": -1, "d": 1}},
		"coordination":    {map[string]int64{"a": 5, "b": 5, "c": 5, "d": 5, "e": 9}, map[string]int{"a": 3, "b": 3, "c": 3, "d": 3, "e": 2}},
		"pair at the top": {map[string]int64{"a": 9, "b": 9, "c": 4, "d": 1}, map[string]int{"a": 2, "b": 2, "c": -1, "d": 1}},
		"two counts":      {map[string]int64{"a": 9, "b": 1}, map[string]int{"a": 2, "b": -1}},
		"alone":           {map[string]int64{"a": 1}, map[string]int{"a": 2}},
		"nobody pressed":  {map[string]int64{}, map[string]int{}},
	} {
		if got := buttonAwards(tc.presses); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

func TestButtonPressBonus(t *testing.T) {
	for presses, want := range map[int64]int{1: 0, 99: 0, 100: 1, 999: 1, 1000: 2, 9999: 2, 10000: 3, 1000000: 3} {
		if got := pressBonus(presses); got != want {
			t.Errorf("pressBonus(%d) = %d, want %d", presses, got, want)
		}
	}
	// The bonus adds to the ranking rules: most +2+2, second -1+1, least +1+0.
	got := buttonAwards(map[string]int64{"a": 1500, "b": 120, "c": 3})
	if want := map[string]int{"a": 4, "c": 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
