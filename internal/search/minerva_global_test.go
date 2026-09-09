package search

import "testing"

func TestSearchMinervaAllPlatformsUsesGlobalIndex(t *testing.T) {
	resetHealthStore()
	t.Cleanup(resetHealthStore)
	svc, _ := seededMinerva(t, true)

	for _, slug := range []string{"", "all"} {
		hits := SearchMinerva(svc, "pokemon heartgold", slug)
		if len(hits) != 1 || hits[0].PlatformSlug != "nds" {
			t.Fatalf("SearchMinerva(%q) hits=%+v, want indexed NDS result", slug, hits)
		}
	}
}
