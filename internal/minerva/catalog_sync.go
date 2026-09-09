package minerva

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *Service) runCatalogSync(ctx context.Context, force bool) (SyncReport, error) {
	s.mu.Lock()
	syncCtx := s.syncCtx
	s.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(syncCtx, cancel)
	defer func() { stop(); cancel() }()

	var report SyncReport
	state := make(map[string]string)
	for _, key := range []string{"catalog_etag", "catalog_last_modified", "catalog_body_sha256"} {
		value, _, err := s.index.State(ctx, key)
		if err != nil {
			return report, err
		}
		state[key] = value
	}

	apiEndpoint, err := url.JoinPath(s.spec.APIURL, "dashboard/latest")
	if err != nil {
		return report, err
	}
	etag, lastModified := state["catalog_etag"], state["catalog_last_modified"]
	if force {
		etag, lastModified = "", ""
	}
	resp, err := s.get(ctx, apiEndpoint, etag, lastModified)
	if err != nil {
		return report, err
	}
	if resp.StatusCode == http.StatusNotModified && !force {
		resp.Body.Close()
		return report, s.index.SetState(ctx, "last_sync", time.Now().UTC().Format(time.RFC3339Nano))
	}
	body, err := readResponse(resp)
	if err != nil {
		return report, err
	}
	catalog, err := parseDashboardLatest(body)
	if err != nil {
		return report, err
	}

	// If the globally cached API response is byte-for-byte unchanged, the
	// torrent catalog and its info hashes are unchanged too.
	if !force && bodyHash(body) == state["catalog_body_sha256"] {
		return report, s.saveCatalogState(ctx, resp, body, catalog)
	}

	slugs := make([]string, 0, len(s.spec.PlatformPaths))
	for slug := range s.spec.PlatformPaths {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		report.Checked++
		entry, foundInCatalog := catalog.findCollection(s.spec.PlatformPaths[slug])
		if !foundInCatalog {
			report.Missing++
			continue
		}
		if len(entry.InfoHash) != 40 {
			return report, fmt.Errorf("minerva: catalog entry %q has invalid info hash %q", entry.Name, entry.InfoHash)
		}
		endpoint, err := torrentCDNURL(s.spec.TorrentsURL, entry.Name)
		if err != nil {
			return report, err
		}
		old, indexed, err := s.index.Collection(ctx, slug)
		if err != nil {
			return report, err
		}
		if indexed && old.TorrentURL == endpoint && strings.EqualFold(old.InfoHash, entry.InfoHash) && !force {
			report.Unchanged++
			continue
		}

		torrentResp, err := s.get(ctx, endpoint, "", "")
		if err != nil {
			return report, err
		}
		if torrentResp.StatusCode == http.StatusNotFound {
			torrentResp.Body.Close()
			report.Missing++
			continue
		}
		torrentBody, err := readResponse(torrentResp)
		if err != nil {
			return report, err
		}
		meta, err := ParseTorrent(torrentBody)
		if err != nil {
			return report, err
		}
		if !strings.EqualFold(meta.InfoHash, entry.InfoHash) {
			return report, fmt.Errorf("minerva: info hash mismatch for %q: API=%s torrent=%s", entry.Name, entry.InfoHash, meta.InfoHash)
		}

		rec := CollectionRecord{
			PlatformSlug:  slug,
			BrowsePath:    s.spec.PlatformPaths[slug],
			BundleVersion: "api",
			TorrentURL:    endpoint,
			InfoHash:      meta.InfoHash,
			ETag:          torrentResp.Header.Get("ETag"),
			LastModified:  torrentResp.Header.Get("Last-Modified"),
			ContentSHA256: bodyHash(torrentBody),
		}
		if err := s.index.ReplaceCollection(ctx, rec, meta.Files); err != nil {
			return report, err
		}
		report.Updated++
		report.Files += len(meta.Files)
	}
	return report, s.saveCatalogState(ctx, resp, body, catalog)
}

func (s *Service) saveCatalogState(ctx context.Context, resp *http.Response, body []byte, catalog dashboardLatest) error {
	states := [][2]string{
		{"catalog_body_sha256", bodyHash(body)},
		{"catalog_etag", resp.Header.Get("ETag")},
		{"catalog_last_modified", resp.Header.Get("Last-Modified")},
		{"catalog_total_torrents", strconv.Itoa(catalog.Metadata.TotalTorrents)},
		{"catalog_global_seeders", strconv.Itoa(catalog.Metadata.GlobalSeeders)},
		{"last_sync", time.Now().UTC().Format(time.RFC3339Nano)},
	}
	for _, state := range states {
		if err := s.index.SetState(ctx, state[0], state[1]); err != nil {
			return err
		}
	}
	return nil
}
