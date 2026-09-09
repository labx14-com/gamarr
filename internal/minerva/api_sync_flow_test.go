package minerva

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gamarr/internal/sources"
)

func TestSyncUsesDashboardCatalogAndTorrentCDN(t *testing.T) {
	torrentBody := []byte(syncTorrent("Game.zip"))
	meta, err := ParseTorrent(torrentBody)
	if err != nil {
		t.Fatal(err)
	}

	mismatch := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/api/dashboard/latest":
			hash := meta.InfoHash
			if mismatch {
				hash = strings.Repeat("f", 40)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"metadata":{"total_torrents":1,"global_seeders":9},"torrents":[{"info_hash":%q,"name":"Minerva_Myrient - No-Intro - Alpha & Beta","size_bytes":12,"seeders":9,"leechers":1}]}`, hash)
		case r.URL.Path == "/torrents/Minerva_Myrient - No-Intro - Alpha & Beta.torrent":
			_, _ = w.Write(torrentBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	spec := sources.MinervaSpec{
		Enabled:     true,
		APIURL:      server.URL + "/v1/api/",
		TorrentsURL: server.URL + "/torrents/",
		PlatformPaths: map[string]string{
			"alpha": "No-Intro/Alpha & Beta/",
		},
	}
	s, err := Open(t.TempDir(), spec)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	report, err := s.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Checked != 1 || report.Updated != 1 || report.Files != 1 {
		t.Fatalf("report = %+v", report)
	}
	rec, found, err := s.index.Collection(context.Background(), "alpha")
	if err != nil || !found {
		t.Fatalf("collection found=%v err=%v", found, err)
	}
	if rec.InfoHash != meta.InfoHash || !strings.Contains(rec.TorrentURL, "/torrents/") {
		t.Fatalf("record = %+v", rec)
	}

	mismatch = true
	if _, err := s.Sync(context.Background(), true); err == nil || !strings.Contains(err.Error(), "info hash mismatch") {
		t.Fatalf("mismatch error = %v", err)
	}
	after, found, err := s.index.Collection(context.Background(), "alpha")
	if err != nil || !found || after != rec {
		t.Fatalf("collection changed after mismatch: found=%v err=%v before=%+v after=%+v", found, err, rec, after)
	}
}
