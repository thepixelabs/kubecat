package cost

import (
	"testing"
)

func TestParseCPU(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"100m", 0.1},
		{"500m", 0.5},
		{"1", 1.0},
		{"2.5", 2.5},
		{"250m", 0.25},
	}
	for _, c := range cases {
		got := parseCPU(c.input)
		if abs(got-c.want) > 0.0001 {
			t.Errorf("parseCPU(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestParseMemoryGB(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"1Gi", 1.0},
		{"512Mi", 0.5},
		{"2Gi", 2.0},
		{"256Mi", 0.25},
		{"1024Mi", 1.0},
	}
	for _, c := range cases {
		got := parseMemoryGB(c.input)
		if abs(got-c.want) > 0.01 {
			t.Errorf("parseMemoryGB(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestEstimate_HeuristicCalculation(t *testing.T) {
	est := New(nil, defaultCPUCostPerCoreHour, defaultMemCostPerGBHour, "USD")

	// 1 core + 1 GB RAM for 1 hour
	result := est.estimate("my-app", "default", 1.0, 1.0)

	// round2 truncates to 2 decimal places; verify arithmetic is consistent.
	if result.CPUCost != round2(defaultCPUCostPerCoreHour*1.0) {
		t.Errorf("CPUCost = %v, want %v", result.CPUCost, round2(defaultCPUCostPerCoreHour))
	}
	if result.MemoryCost != round2(defaultMemCostPerGBHour*1.0) {
		t.Errorf("MemoryCost = %v, want %v", result.MemoryCost, round2(defaultMemCostPerGBHour))
	}
	rawTotal := defaultCPUCostPerCoreHour + defaultMemCostPerGBHour // 0.054
	if abs(result.TotalCost-round2(rawTotal)) > 0.001 {
		t.Errorf("TotalCost = %v, want ~%v", result.TotalCost, round2(rawTotal))
	}
	if result.MonthlyTotal <= 0 {
		t.Errorf("MonthlyTotal should be positive, got %v", result.MonthlyTotal)
	}
	if result.Source != SourceHeuristic {
		t.Errorf("Source = %q, want %q", result.Source, SourceHeuristic)
	}
	if result.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", result.Currency)
	}
}

func TestEstimate_ZeroResources_ZeroCost(t *testing.T) {
	est := New(nil, 0, 0, "")
	result := est.estimate("idle-app", "default", 0, 0)
	if result.TotalCost != 0 {
		t.Errorf("expected zero cost for zero resources, got %v", result.TotalCost)
	}
}

func TestInferWorkload(t *testing.T) {
	cases := []struct {
		pod  string
		want string
	}{
		{"my-app-7d9f6b8c4-xk2p9", "my-app"},
		{"nginx-deployment-5d4b8f7c9-abc12", "nginx-deployment"},
		{"simple", "simple"},
		{"two-parts", "two-parts"},
	}
	for _, c := range cases {
		got := inferWorkload(c.pod)
		if got != c.want {
			t.Errorf("inferWorkload(%q) = %q, want %q", c.pod, got, c.want)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
