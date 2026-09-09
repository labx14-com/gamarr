package config

import "testing"

func TestQBitDefaultCategoryIsGamarr(t *testing.T) {
	t.Setenv("QB_CATEGORY", "")
	cfg := Load()
	if cfg.QBCategory != "gamarr" {
		t.Fatalf("QBCategory=%q, want gamarr", cfg.QBCategory)
	}
}
