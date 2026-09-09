package download

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectiveImportUsesLiveContentPathAfterQBitMove(t *testing.T) {
	m, q := newSelectiveTest(t)
	q.exists = true

	actualParent := filepath.Join(t.TempDir(), "incomplete", "games")
	actualRoot := filepath.Join(actualParent, "Collection")
	actualFile := filepath.Join(actualRoot, "HeartGold.nds")
	writeFileT(t, actualFile, []byte("rom"))

	q.mu.Lock()
	q.torrent.SavePath = filepath.Join(t.TempDir(), "final", "games")
	q.torrent.ContentPath = actualRoot
	q.torrent.State = "stoppedUP"
	q.torrent.Progress = 1
	q.mu.Unlock()

	id, err := m.DownloadSelectiveTorrent(
		"https://example.test/collection.torrent", "abcdef1234", 7,
		"Collection/HeartGold.nds", 3, "HeartGold", "DS", "nds", false,
	)
	if err != nil {
		t.Fatal(err)
	}
	job := selectiveJobDone(t, m, id)
	if job["status"] != "completed" {
		t.Fatalf("selection did not import from content_path: %#v", job)
	}
	dest := filepath.Join(m.cfg.GamesRomsPath, "nds", "HeartGold.nds")
	if data, err := os.ReadFile(dest); err != nil || string(data) != "rom" {
		t.Fatalf("imported payload=%q err=%v", data, err)
	}
	if _, err := os.Stat(actualFile); err != nil {
		t.Fatalf("qB payload was not preserved: %v", err)
	}
}
