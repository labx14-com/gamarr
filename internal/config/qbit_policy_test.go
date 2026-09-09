package config

import "testing"

func TestQBitDefaultsDelegatePathsToCategory(t *testing.T) {
	t.Setenv("QB_CATEGORY", "")
	t.Setenv("QB_SAVE_PATH", "")
	cfg := Load()
	if cfg.QBCategory != "gamarr" {
		t.Fatalf("QBCategory=%q, want gamarr", cfg.QBCategory)
	}
	if cfg.QBSavePath != "" {
		t.Fatalf("QBSavePath=%q, want empty so qB manages paths", cfg.QBSavePath)
	}
}
