// Package qbit is a minimal qBittorrent Web API client used to add, inspect,
// and manage torrents.
package qbit

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Torrent represents a qBittorrent torrent entry.
type Torrent struct {
	Name        string  `json:"name"`
	Hash        string  `json:"hash"`
	Progress    float64 `json:"progress"`
	State       string  `json:"state"`
	TotalSize   int64   `json:"total_size"`
	DLSpeed     int64   `json:"dlspeed"`
	ETA         int     `json:"eta"`
	SavePath    string  `json:"save_path"`
	ContentPath string  `json:"content_path"`
}

// TorrentFile represents a file within a torrent.
type TorrentFile struct {
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Priority int     `json:"priority"`
	Index    int     `json:"index"`
	Progress float64 `json:"progress"`
}

// Client is a qBittorrent API client. Auth is either a session cookie
// (username/password) or a Bearer API key (qBittorrent ≥ 5.2).
type Client struct {
	mu            sync.Mutex
	client        *http.Client
	baseURL       string
	user          string
	pass          string
	apiKey        string
	authenticated bool
}

// New creates a cookie-auth qBittorrent client.
func New(baseURL, user, pass string) *Client {
	return newClient(baseURL, user, pass, "")
}

// NewWithAPIKey creates a Bearer-auth qBittorrent client (qBittorrent ≥ 5.2 WebAPI).
func NewWithAPIKey(baseURL, apiKey string) *Client {
	return newClient(baseURL, "", "", apiKey)
}

func newClient(baseURL, user, pass, apiKey string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		client:  &http.Client{Jar: jar, Timeout: 15 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		user:    user, pass: pass, apiKey: apiKey,
	}
}

func (c *Client) Login() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.login()
}

func (c *Client) login() bool {
	if c.apiKey != "" {
		return c.probeAPIKey()
	}
	data := url.Values{"username": {c.user}, "password": {c.pass}}
	resp, err := c.client.PostForm(c.baseURL+"/api/v2/auth/login", data)
	if err != nil {
		slog.Error("qBittorrent login failed", "error", err)
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	c.authenticated = string(body) == "Ok." || (resp.StatusCode == http.StatusNoContent && len(body) == 0)
	return c.authenticated
}

func (c *Client) probeAPIKey() bool {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v2/app/version", nil)
	if err != nil {
		c.authenticated = false
		return false
	}
	c.setAuth(req)
	resp, err := c.client.Do(req)
	if err != nil {
		slog.Error("qBittorrent API key probe failed", "error", err)
		c.authenticated = false
		return false
	}
	defer resp.Body.Close()
	c.authenticated = is2xx(resp.StatusCode)
	if !c.authenticated {
		slog.Error("qBittorrent API key rejected", "status", resp.StatusCode)
	}
	return c.authenticated
}

func (c *Client) setAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

func (c *Client) canReauth() bool { return c.apiKey == "" }

func (c *Client) postForm(endpoint string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", c.baseURL+endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.setAuth(req)
	return c.client.Do(req)
}

func is2xx(code int) bool { return code >= 200 && code < 300 }

func addAccepted(statusCode int, body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "Ok." {
		return true
	}
	if !is2xx(statusCode) {
		return false
	}
	var result struct {
		SuccessCount *int `json:"success_count"`
		PendingCount *int `json:"pending_count"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.SuccessCount != nil {
		accepted := *result.SuccessCount
		if result.PendingCount != nil {
			accepted += *result.PendingCount
		}
		return accepted > 0
	}
	return trimmed == ""
}

func (c *Client) ensureAuth() {
	if !c.authenticated {
		c.login()
	}
}

// torrentAddValues deliberately ignores the caller's legacy savePath. Torrent
// placement belongs to qBittorrent (category/default save path and incomplete
// path), while Gamarr's QBSavePath remains available as staging for DDL and
// fallback clients that still need a filesystem path.
func torrentAddValues(torrentURL, _ string, category string) url.Values {
	data := url.Values{"urls": {torrentURL}}
	if strings.TrimSpace(category) != "" {
		data.Set("category", category)
	}
	return data
}

// AddTorrent adds a torrent to qBittorrent without overriding qB's path policy.
func (c *Client) AddTorrent(torrentURL, title, savePath, category string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureAuth()

	data := torrentAddValues(torrentURL, savePath, category)
	resp, err := c.postForm("/api/v2/torrents/add", data)
	if err != nil {
		slog.Error("qBittorrent add torrent failed", "error", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 && c.canReauth() {
		c.login()
		resp2, err := c.postForm("/api/v2/torrents/add", data)
		if err != nil {
			return false
		}
		defer resp2.Body.Close()
		body, _ := io.ReadAll(resp2.Body)
		return addAccepted(resp2.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	return addAccepted(resp.StatusCode, body)
}

// AddTorrentPaused adds a torrent without starting it. stopped supports
// current qBittorrent versions while paused keeps compatibility with older
// releases that accepted only the legacy field. title is intentionally not
// used as qB's display name: Minerva callers pass a game title, while one
// collection torrent is shared by many games and is renamed after metadata is
// available.
func (c *Client) AddTorrentPaused(torrentURL, title, savePath, category string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureAuth()

	data := torrentAddValues(torrentURL, savePath, category)
	data.Set("stopped", "true")
	data.Set("paused", "true")
	resp, err := c.postForm("/api/v2/torrents/add", data)
	if err != nil {
		slog.Error("qBittorrent add paused torrent failed", "error", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden && c.canReauth() {
		c.login()
		resp2, err := c.postForm("/api/v2/torrents/add", data)
		if err != nil {
			return false
		}
		defer resp2.Body.Close()
		body, _ := io.ReadAll(resp2.Body)
		return addAccepted(resp2.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
	return addAccepted(resp.StatusCode, body)
}

// GetTorrents returns torrents, optionally filtered by category. qB's
// content_path follows a torrent while it is moved between incomplete and
// completed locations; when present its parent is therefore the live payload
// root used by selective Minerva imports.
func (c *Client) GetTorrents(category string) ([]Torrent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureAuth()

	u := c.baseURL + "/api/v2/torrents/info"
	if category != "" {
		u += "?category=" + url.QueryEscape(category)
	}
	torrents, err := c.doGetJSON(u)
	if err != nil {
		return nil, err
	}
	for i := range torrents {
		if strings.TrimSpace(torrents[i].ContentPath) != "" {
			torrents[i].SavePath = filepath.Dir(torrents[i].ContentPath)
		}
	}
	return torrents, nil
}

func (c *Client) GetTorrentFiles(hash string) []TorrentFile {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureAuth()
	return c.getTorrentFilesLocked(hash)
}

func (c *Client) getTorrentFilesLocked(hash string) []TorrentFile {
	u := fmt.Sprintf("%s/api/v2/torrents/files?hash=%s", c.baseURL, url.QueryEscape(hash))
	resp, err := c.doGet(u)
	if err != nil {
		slog.Error("qBittorrent file list unreadable", "hash", hash, "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 && c.canReauth() {
		c.login()
		resp2, err := c.doGet(u)
		if err != nil {
			return nil
		}
		defer resp2.Body.Close()
		if !is2xx(resp2.StatusCode) {
			return nil
		}
		return decodeTorrentFiles(hash, resp2.Body)
	}
	if !is2xx(resp.StatusCode) {
		return nil
	}
	return decodeTorrentFiles(hash, resp.Body)
}

func decodeTorrentFiles(hash string, body io.Reader) []TorrentFile {
	var files []TorrentFile
	if err := json.NewDecoder(body).Decode(&files); err != nil {
		slog.Error("qBittorrent file list undecodable", "hash", hash, "error", err)
		return nil
	}
	return files
}

func (c *Client) DeleteTorrent(hash string, deleteFiles bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureAuth()

	delStr := "false"
	if deleteFiles {
		delStr = "true"
	}
	data := url.Values{"hashes": {hash}, "deleteFiles": {delStr}}
	resp, err := c.postForm("/api/v2/torrents/delete", data)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 && c.canReauth() {
		c.login()
		resp2, err := c.postForm("/api/v2/torrents/delete", data)
		if err != nil {
			return false
		}
		defer resp2.Body.Close()
		return is2xx(resp2.StatusCode)
	}
	return is2xx(resp.StatusCode)
}

func (c *Client) StopTorrent(hash string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureAuth()
	data := url.Values{"hashes": {hash}}
	status := c.postWithReauth("/api/v2/torrents/stop", data)
	if status == http.StatusNotFound {
		status = c.postWithReauth("/api/v2/torrents/pause", data)
	}
	return is2xx(status)
}

func commonCollectionName(files []TorrentFile) string {
	var common []string
	for _, file := range files {
		dir := path.Dir(strings.TrimSpace(file.Name))
		if dir == "." || dir == "/" {
			return ""
		}
		parts := strings.Split(strings.Trim(dir, "/"), "/")
		if len(common) == 0 {
			common = append([]string(nil), parts...)
			continue
		}
		n := len(common)
		if len(parts) < n {
			n = len(parts)
		}
		j := 0
		for j < n && common[j] == parts[j] {
			j++
		}
		common = common[:j]
		if len(common) == 0 {
			return ""
		}
	}
	if len(common) == 0 {
		return ""
	}
	return strings.TrimSpace(common[len(common)-1])
}

// SetFilePriority updates file priorities. The single wanted-file transition
// used by Minerva is also the first point where qB has the full file list, so
// it is the safe place to rename the shared torrent to its collection rather
// than to whichever game happened to create it.
func (c *Client) SetFilePriority(hash string, ids []int, priority int) bool {
	if len(ids) == 0 {
		return false
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureAuth()
	data := url.Values{"hash": {hash}, "id": {strings.Join(parts, "|")}, "priority": {strconv.Itoa(priority)}}
	if !is2xx(c.postWithReauth("/api/v2/torrents/filePrio", data)) {
		return false
	}
	if priority > 0 && len(ids) == 1 {
		if name := commonCollectionName(c.getTorrentFilesLocked(hash)); name != "" {
			rename := url.Values{"hash": {hash}, "name": {name}}
			if !is2xx(c.postWithReauth("/api/v2/torrents/rename", rename)) {
				slog.Warn("qBittorrent collection rename failed", "hash", hash, "name", name)
			}
		}
	}
	return true
}

func (c *Client) StartTorrent(hash string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureAuth()
	data := url.Values{"hashes": {hash}}
	status := c.postWithReauth("/api/v2/torrents/start", data)
	if status == http.StatusNotFound {
		status = c.postWithReauth("/api/v2/torrents/resume", data)
	}
	return is2xx(status)
}

func (c *Client) postWithReauth(endpoint string, data url.Values) int {
	resp, err := c.postForm(endpoint, data)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden || !c.canReauth() {
		return resp.StatusCode
	}
	c.login()
	resp2, err := c.postForm(endpoint, data)
	if err != nil {
		return 0
	}
	defer resp2.Body.Close()
	return resp2.StatusCode
}

func (c *Client) doGet(u string) (*http.Response, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	return c.client.Do(req)
}

func (c *Client) doGetJSON(u string) ([]Torrent, error) {
	resp, err := c.doGet(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 && c.canReauth() {
		c.login()
		resp2, err := c.doGet(u)
		if err != nil {
			return nil, err
		}
		defer resp2.Body.Close()
		if !is2xx(resp2.StatusCode) {
			return nil, fmt.Errorf("HTTP %d", resp2.StatusCode)
		}
		var torrents []Torrent
		err = json.NewDecoder(resp2.Body).Decode(&torrents)
		return torrents, err
	}
	if !is2xx(resp.StatusCode) {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var torrents []Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	return torrents, err
}
