package search

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	ghclient "skillshare/internal/github"
)

// ErrSkillNotFound is returned when SKILL.md cannot be found at the given source.
var ErrSkillNotFound = errors.New("skill not found")

// SkillPreview contains the full SKILL.md content + parsed frontmatter metadata
// for previewing a remote skill before installation.
type SkillPreview struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	License     string   `json:"license,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Content     string   `json:"content"`
	Source      string   `json:"source"`
	Stars       int      `json:"stars"`
	Owner       string   `json:"owner"`
	Repo        string   `json:"repo"`
}

// FetchSkillContent fetches the full SKILL.md from a GitHub repository and
// returns parsed frontmatter metadata along with the raw content.
// The path parameter is the subdirectory within the repo (empty or "." for root).
func FetchSkillContent(client *http.Client, owner, repo, path, branch string) (*SkillPreview, error) {
	skillPath := "SKILL.md"
	if path != "" && path != "." {
		skillPath = path + "/SKILL.md"
	}

	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s",
		owner, repo, url.PathEscape(skillPath),
	)
	if branch != "" {
		apiURL += "?ref=" + url.QueryEscape(branch)
	}

	req, err := ghclient.NewRequest(apiURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := ghclient.CheckRateLimit(resp); err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound && path != "" && path != "." {
		// Direct path failed — try finding SKILL.md via Git Tree API.
		// Hub sources use install shorthands (e.g. "owner/repo/critique")
		// where the actual file may be at "source/skills/critique/SKILL.md".
		if resolved := resolveSkillPath(client, owner, repo, path, branch); resolved != "" {
			return FetchSkillContent(client, owner, repo, resolved, branch)
		}
		return nil, fmt.Errorf("%w: %s/%s/%s", ErrSkillNotFound, owner, repo, path)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s/%s", ErrSkillNotFound, owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var content gitHubContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return nil, err
	}
	if content.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", content.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return nil, err
	}

	body := string(decoded)

	// Build source string
	source := owner + "/" + repo
	if path != "" && path != "." {
		source = source + "/" + path
	}

	preview := &SkillPreview{
		Name:        parseFrontmatterField(body, "name"),
		Description: parseFrontmatterField(body, "description"),
		License:     parseFrontmatterField(body, "license"),
		Content:     body,
		Source:      source,
		Owner:       owner,
		Repo:        repo,
	}

	// Parse tags (comma-separated or YAML list on one line)
	if tagsRaw := parseFrontmatterField(body, "tags"); tagsRaw != "" {
		tagsRaw = strings.Trim(tagsRaw, "[]")
		for t := range strings.SplitSeq(tagsRaw, ",") {
			t = strings.TrimSpace(t)
			t = strings.Trim(t, `"'`)
			if t != "" {
				preview.Tags = append(preview.Tags, t)
			}
		}
	}

	// Fetch star count (best-effort, don't fail on error)
	if stars, err := fetchRepoStars(client, owner, repo); err == nil {
		preview.Stars = stars
	}

	return preview, nil
}

// resolveSkillPath uses the Git Tree API to find the actual path of a
// SKILL.md that matches the given skill name. Hub sources use install
// shorthands where the path segment is the skill name, not the repo path.
// Returns the resolved directory path (e.g. "source/skills/critique") or "".
func resolveSkillPath(client *http.Client, owner, repo, skillName, branch string) string {
	ref := "HEAD"
	if branch != "" {
		ref = branch
	}
	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
		owner, repo, ref,
	)

	req, err := ghclient.NewRequest(apiURL)
	if err != nil {
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return ""
	}

	// Look for {skillName}/SKILL.md — prefer paths under "source/" (original source)
	suffix := "/" + skillName + "/SKILL.md"
	var best string
	for _, entry := range tree.Tree {
		if entry.Type != "blob" {
			continue
		}
		if !strings.HasSuffix(entry.Path, suffix) {
			continue
		}
		// Strip the trailing /SKILL.md to get the directory path
		dir := strings.TrimSuffix(entry.Path, "/SKILL.md")
		if strings.HasPrefix(entry.Path, "source/") {
			return dir // prefer source/ paths
		}
		if best == "" {
			best = dir
		}
	}
	return best
}
