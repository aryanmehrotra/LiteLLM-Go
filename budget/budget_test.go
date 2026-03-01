package budget

import (
	"testing"
)

func TestAlertThresholds(t *testing.T) {
	if len(alertThresholds) != 3 {
		t.Fatalf("expected 3 alert thresholds, got %d", len(alertThresholds))
	}

	expected := []float64{0.50, 0.80, 1.00}
	for i, want := range expected {
		if alertThresholds[i] != want {
			t.Errorf("threshold[%d] = %.2f, want %.2f", i, alertThresholds[i], want)
		}
	}
}

func TestAlertThresholds_Sorted(t *testing.T) {
	for i := 1; i < len(alertThresholds); i++ {
		if alertThresholds[i] <= alertThresholds[i-1] {
			t.Errorf("thresholds not in ascending order: %.2f >= %.2f at index %d", alertThresholds[i-1], alertThresholds[i], i)
		}
	}
}
