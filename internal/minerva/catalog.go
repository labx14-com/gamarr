package minerva

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type dashboardMetadata struct {
	TotalTorrents int `json:"total_torrents"`
	GlobalSeeders int `json:"global_seeders"`
}

type dashboardTorrent struct {
	InfoHash  string `json:"info_hash"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	Seeders   int    `json:"seeders"`
	Leechers  int    `json:"leechers"`
}

type dashboardLatest struct {
	Metadata         dashboardMetadata  `json:"metadata"`
	PreviousMetadata dashboardMetadata  `json:"previous_metadata"`
	Torrents         []dashboardTorrent `json:"torrents"`
}

func parseDashboardLatest(body []byte) (dashboardLatest, error) {
	var catalog dashboardLatest
	if err := json.Unmarshal(body, &catalog); err != nil {
		return dashboardLatest{}, fmt.Errorf("minerva: decode dashboard catalog: %w", err)
	}
	return catalog, nil
}

func collectionCatalogName(browsePath string) string {
	name := strings.ReplaceAll(strings.Trim(strings.TrimSpace(browsePath), "/"), "/", " - ")
	return "Minerva_Myrient - " + name
}

func (c dashboardLatest) findCollection(browsePath string) (dashboardTorrent, bool) {
	want := collectionCatalogName(browsePath)
	for _, torrent := range c.Torrents {
		if torrent.Name == want {
			return torrent, true
		}
	}
	return dashboardTorrent{}, false
}

func torrentCDNURL(baseURL, torrentName string) (string, error) {
	return url.JoinPath(baseURL, torrentName+".torrent")
}
