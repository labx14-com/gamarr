package qbit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddTorrentPausedOmitsEmptySavePathAndRenamesCollection(t *testing.T) {
	var addForm map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/add":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			addForm = r.Form
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "admin", "pass")
	if !c.AddTorrentPaused("https://example.test/collection.torrent", "Nintendo - Nintendo DS (Decrypted)", "", "gamarr") {
		t.Fatal("paused add failed")
	}
	if _, exists := addForm["savepath"]; exists {
		t.Fatalf("savepath must be omitted when qB manages paths: %#v", addForm["savepath"])
	}
	if got := addForm.Get("category"); got != "gamarr" {
		t.Fatalf("category=%q, want gamarr", got)
	}
	if got := addForm.Get("rename"); got != "Nintendo - Nintendo DS (Decrypted)" {
		t.Fatalf("rename=%q, want collection name", got)
	}
}
