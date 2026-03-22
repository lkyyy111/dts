package utils

import "testing"

func TestAdd(t *testing.T) {
	ans := Add(1, 2)

	if ans != 3 {
		t.Errorf("expected: 3, got: %d", ans)
	}
}