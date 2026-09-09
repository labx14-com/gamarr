package minerva

import (
	"testing"
)

func TestParseDashboardLatest(t *testing.T) {
	body := []byte(`{
		"metadata":{"total_torrents":2,"global_seeders":42},
		"previous_metadata":{"total_torrents":1},
		"torrents":[
			{"info_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","name":"Minerva_Myrient - No-Intro - Nintendo - Game Boy Advance","size_bytes":1234,"seeders":12,"leechers":3},
			{"info_hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","name":"Minerva_Myrient - Redump - Sony - PlayStation","size_bytes":5678,"seeders":0,"leechers":1}
		]
	}`)

	catalog, err := parseDashboardLatest(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Torrents) != 2 {
		t.Fatalf("torrent count = %d, want 2", len(catalog.Torrents))
	}
	if catalog.Metadata.TotalTorrents != 2 || catalog.Metadata.GlobalSeeders != 42 {
		t.Fatalf("metadata = %+v", catalog.Metadata)
	}
	got := catalog.Torrents[0]
	if got.InfoHash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || got.Name != "Minerva_Myrient - No-Intro - Nintendo - Game Boy Advance" || got.SizeBytes != 1234 || got.Seeders != 12 || got.Leechers != 3 {
		t.Fatalf("torrent = %+v", got)
	}
}

func TestCatalogMatchesCollectionAndBuildsCDNURL(t *testing.T) {
	catalog := dashboardLatest{Torrents: []dashboardTorrent{{
		InfoHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name:     "Minerva_Myrient - No-Intro - Nintendo - Game Boy Advance",
	}}}

	entry, ok := catalog.findCollection("No-Intro/Nintendo - Game Boy Advance/")
	if !ok {
		t.Fatal("collection not found")
	}
	if entry.InfoHash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("info hash = %q", entry.InfoHash)
	}

	u, err := torrentCDNURL("https://cdn.minerva-archive.org/torrents/", entry.Name)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://cdn.minerva-archive.org/torrents/Minerva_Myrient%20-%20No-Intro%20-%20Nintendo%20-%20Game%20Boy%20Advance.torrent"
	if u != want {
		t.Fatalf("url = %q, want %q", u, want)
	}
}
