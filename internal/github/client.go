// Package github provides a GitHub Releases API client for Kompa.
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	apiBase    = "https://api.github.com"
	userAgent  = "kompa/1.0 (+https://github.com/kompa-tbm/kompa)"
	acceptJSON = "application/vnd.github+json"
)

// Asset represents a file attached to a GitHub release.
type Asset struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	ContentType        string    `json:"content_type"`
	Size               int64     `json:"size"`
	BrowserDownloadURL string    `json:"browser_download_url"`
	CreatedAt          time.Time `json:"created_at"`
}

// Release represents a GitHub release.
type Release struct {
	ID         int64     `json:"id"`
	TagName    string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	CreatedAt  time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
	Assets     []Asset   `json:"assets"`
	HTMLURL    string    `json:"html_url"`
}

// FindAsset returns the asset with the given filename, or nil.
func (r *Release) FindAsset(name string) *Asset {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}

// Client is a minimal GitHub REST API v3 client.
type Client struct {
	repo       string
	token      string
	httpClient *http.Client
}

// NewClient returns a new Client for the given "owner/repo" string.
// token may be empty for unauthenticated access (lower rate limits).
func NewClient(repo, token string) *Client {
	return &Client{
		repo:  repo,
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetTimeout overrides the default HTTP timeout.
func (c *Client) SetTimeout(d time.Duration) {
	c.httpClient.Timeout = d
}

// LatestRelease returns the most recently published non-draft, non-prerelease release.
func (c *Client) LatestRelease() (*Release, error) {
	path := fmt.Sprintf("/repos/%s/releases/latest", c.repo)
	var r Release
	if err := c.get(path, &r); err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	return &r, nil
}

// ListReleases returns all releases for the repository (paginated, up to 100).
func (c *Client) ListReleases() ([]*Release, error) {
	path := fmt.Sprintf("/repos/%s/releases?per_page=100", c.repo)
	var releases []*Release
	if err := c.get(path, &releases); err != nil {
		return nil, fmt.Errorf("listing releases: %w", err)
	}
	return releases, nil
}

// GetReleaseByTag returns the release for the given tag name.
func (c *Client) GetReleaseByTag(tag string) (*Release, error) {
	path := fmt.Sprintf("/repos/%s/releases/tags/%s", c.repo, url.PathEscape(tag))
	var r Release
	if err := c.get(path, &r); err != nil {
		return nil, fmt.Errorf("fetching release %s: %w", tag, err)
	}
	return &r, nil
}

// NextReleaseTag determines the next sequential vN release tag.
// It lists all existing tags, finds the highest vN, and returns v(N+1).
// If no tags exist, it returns "v1".
func (c *Client) NextReleaseTag() (string, error) {
	tags, err := c.listTags()
	if err != nil {
		return "", fmt.Errorf("listing tags for next release: %w", err)
	}

	max := 0
	for _, tag := range tags {
		if !strings.HasPrefix(tag, "v") {
			continue
		}
		n, err := strconv.Atoi(tag[1:])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("v%d", max+1), nil
}

// ReleaseTag is the list of tag names in the repository.
type tagListEntry struct {
	Name string `json:"name"`
}

func (c *Client) listTags() ([]string, error) {
	path := fmt.Sprintf("/repos/%s/tags?per_page=100", c.repo)
	var entries []tagListEntry
	if err := c.get(path, &entries); err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names, nil
}

// DownloadAsset fetches the content of an asset by URL, writing to w.
// It returns the total bytes written.
func (c *Client) DownloadAsset(downloadURL string, w io.Writer) (int64, error) {
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return 0, fmt.Errorf("creating download request: %w", err)
	}
	c.addHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("downloading %s: %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("downloading %s: HTTP %d", downloadURL, resp.StatusCode)
	}

	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return n, fmt.Errorf("reading download body: %w", err)
	}
	return n, nil
}

// FindManifestURL returns the browser download URL for a manifest.json asset
// in the given release.
func (c *Client) FindManifestURL(r *Release) (string, error) {
	asset := r.FindAsset("manifest.json")
	if asset == nil {
		return "", fmt.Errorf("release %s has no manifest.json asset", r.TagName)
	}
	return asset.BrowserDownloadURL, nil
}

// LatestReleaseWithManifest returns the latest release that contains a manifest.json asset.
func (c *Client) LatestReleaseWithManifest() (*Release, error) {
	releases, err := c.ListReleases()
	if err != nil {
		return nil, err
	}

	// Sort by published date descending.
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].PublishedAt.After(releases[j].PublishedAt)
	})

	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		if r.FindAsset("manifest.json") != nil {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no release with a manifest.json found in repository %s", c.repo)
}

// FetchManifestContent downloads and returns the raw content of the manifest.json
// from the given release.
func (c *Client) FetchManifestContent(r *Release) ([]byte, error) {
	manifestURL, err := c.FindManifestURL(r)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating manifest request: %w", err)
	}
	c.addHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest from %s: %w", manifestURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching manifest: HTTP %d from %s", resp.StatusCode, manifestURL)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading manifest body: %w", err)
	}
	return data, nil
}

// GetRateLimit returns the current GitHub API rate limit status.
func (c *Client) GetRateLimit() (remaining, limit int, err error) {
	type rateLimitResponse struct {
		Resources struct {
			Core struct {
				Remaining int `json:"remaining"`
				Limit     int `json:"limit"`
			} `json:"core"`
		} `json:"resources"`
	}
	var r rateLimitResponse
	if err := c.get("/rate_limit", &r); err != nil {
		return 0, 0, fmt.Errorf("checking rate limit: %w", err)
	}
	return r.Resources.Core.Remaining, r.Resources.Core.Limit, nil
}

// get performs a GET request to the GitHub API and decodes the JSON response.
func (c *Client) get(path string, out interface{}) error {
	reqURL := apiBase + path
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}
	c.addHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Try to give an informative message for common network errors.
		if os.IsTimeout(err) {
			return fmt.Errorf("GitHub API request timed out (%s): %w", path, err)
		}
		return fmt.Errorf("GitHub API request failed (%s): %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body for %s: %w", path, err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return &NotFoundError{Path: path}
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitError{StatusCode: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API returned HTTP %d for %s: %s", resp.StatusCode, path, truncate(string(body), 200))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding JSON response for %s: %w", path, err)
	}
	return nil
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", acceptJSON)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// NotFoundError is returned when the API responds with 404.
type NotFoundError struct {
	Path string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("GitHub resource not found: %s", e.Path)
}

// IsNotFound returns true when err is a *NotFoundError.
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// RateLimitError is returned when GitHub rate-limits the request.
type RateLimitError struct {
	StatusCode int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("GitHub API rate limit exceeded (HTTP %d); set GITHUB_TOKEN or wait before retrying", e.StatusCode)
}

// IsRateLimit returns true when err is a *RateLimitError.
func IsRateLimit(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
