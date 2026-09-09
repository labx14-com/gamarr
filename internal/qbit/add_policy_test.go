package qbit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAddTorrentPausedDelegatesSavePathToQBit(t *testing.T) {
	var addForm url.Values
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
	if !c.AddTorrentPaused("https://example.test/collection.torrent", "game title", "/legacy/gamarr/path", "gamarr") {
		t.Fatal("paused add failed")
	}
	if _, exists := addForm["savepath"]; exists {
		t.Fatalf("savepath must be omitted so qB manages paths: %#v", addForm["savepath"])
	}
	if got := addForm.Get("category"); got != "gamarr" {
		t.Fatalf("category=%q, want gamarr", got)
	}
	if got := addForm.Get("rename"); got != "" {
		t.Fatalf("new Minerva torrent must not be named after the game: %q", got)
	}
}

func TestSetFilePriorityRenamesTorrentToCollection(t *testing.T) {
	var renamed string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/filePrio":
			w.WriteHeader(http.StatusNoContent)
		case "/api/v2/torrents/files":
			_ = json.NewEncoder(w).Encode([]TorrentFile{
				{Index: 7, Name: "Minerva_Myrient/No-Intro/Nintendo - Nintendo DS (Decrypted)/HeartGold.zip"},
				{Index: 8, Name: "Minerva_Myrient/No-Intro/Nintendo - Nintendo DS (Decrypted)/Mario Kart DS.zip"},
			})
		case "/api/v2/torrents/rename":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			renamed = r.Form.Get("name")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "admin", "pass")
	if !c.SetFilePriority("abc", []int{7}, 7) {
		t.Fatal("file priority failed")
	}
	if renamed != "Nintendo - Nintendo DS (Decrypted)" {
		t.Fatalf("renamed=%q, want collection name", renamed)
	}
}

func TestGetTorrentsUsesLiveContentParentAsPayloadRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(w).Encode([]Torrent{{
				Hash: "abc", SavePath: "/data/torrents/games",
				ContentPath: "/data/torrents/incomplete/games/Minerva_Myrient",
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "admin", "pass")
	torrents, err := c.GetTorrents("")
	if err != nil || len(torrents) != 1 {
		t.Fatalf("GetTorrents=%+v err=%v", torrents, err)
	}
	if got := torrents[0].SavePath; got != "/data/torrents/incomplete/games" {
		t.Fatalf("SavePath=%q, want live payload root", got)
	}
}
