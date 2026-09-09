package minerva

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchAcrossAllPlatformsWhenPlatformIsEmpty(t *testing.T) {
	ctx := context.Background()
	idx, err := OpenIndex(filepath.Join(t.TempDir(), "minerva.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = idx.Close() })

	for _, rec := range []struct {
		slug string
		hash string
		name string
	}{
		{slug: "gba", hash: strings.Repeat("a", 40), name: "Pokemon Emerald.gba"},
		{slug: "nds", hash: strings.Repeat("b", 40), name: "Pokemon HeartGold.nds"},
	} {
		if err := idx.ReplaceCollection(ctx, CollectionRecord{
			PlatformSlug: rec.slug, BrowsePath: "/" + rec.slug, BundleVersion: "1",
			TorrentURL: "https://example.test/" + rec.slug + ".torrent", InfoHash: rec.hash,
			ContentSHA256: "fixture-" + rec.slug,
		}, []FileMeta{{Index: 0, Path: rec.name, Name: rec.name, Size: 1}}); err != nil {
			t.Fatal(err)
		}
	}

	hits, err := idx.Search(ctx, "pokemon", "", 20)
	if err != nil {
		t.Fatalf("global search failed: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("global hits=%+v, want both platforms", hits)
	}
	if hits[0].PlatformSlug == hits[1].PlatformSlug {
		t.Fatalf("global search did not cross platforms: %+v", hits)
	}
}
