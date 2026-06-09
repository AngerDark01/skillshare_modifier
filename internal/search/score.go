package search

import (
	"math"
	"strings"
)

// Scoring weights for multi-signal relevance ranking.
// Stars weight is high because it's the strongest proxy for "someone actually uses this."
// Name match still matters but shouldn't let 0-star repos dominate results.
const (
	weightName        = 0.28
	weightDescription = 0.18
	weightStars       = 0.24
	weightSource      = 0.12
	weightRepository  = 0.18
)

// scoreResult computes a composite relevance score for a search result.
// When query is empty (browse mode), scoring is stars-only to preserve
// the existing behavior of showing popular skills first.
func scoreResult(r SearchResult, query string) float64 {
	if query == "" {
		return normalizeStars(r.Stars)
	}

	name := nameMatchScore(r.Name, query)
	desc := descriptionMatchScore(r.Description, query)
	stars := normalizeStars(r.Stars)
	source := sourceQualityScore(r)
	repository := repositoryMatchScore(r, query)

	return name*weightName + desc*weightDescription + stars*weightStars + source*weightSource + repository*weightRepository
}

// nameMatchScore scores how well a skill name matches the query.
//
//	exact match      → 1.0
//	name contains q  → 0.7
//	word boundary    → 0.6
//	no match         → 0.0
func nameMatchScore(name, query string) float64 {
	nl := strings.ToLower(name)
	ql := strings.ToLower(query)

	if nl == ql {
		return 1.0
	}
	if strings.Contains(nl, ql) {
		return 0.7
	}

	// Word boundary: query matches a hyphen/underscore-separated segment
	for _, seg := range strings.FieldsFunc(nl, func(r rune) bool {
		return r == '-' || r == '_' || r == '/'
	}) {
		if seg == ql {
			return 0.6
		}
	}

	return 0.0
}

// descriptionMatchScore scores how many query words appear in the description.
// Returns the ratio of matched words to total query words.
func descriptionMatchScore(desc, query string) float64 {
	if desc == "" || query == "" {
		return 0.0
	}

	dl := strings.ToLower(desc)
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return 0.0
	}

	matched := 0
	for _, w := range words {
		if strings.Contains(dl, w) {
			matched++
		}
	}

	return float64(matched) / float64(len(words))
}

// normalizeStars maps star count to [0, 1] using log10 scale.
// The divisor of 3.0 gives better differentiation in the 5-50 star range
// where most useful skills live:
// 0 → 0, 1 → 0, 5 → 0.23, 10 → 0.33, 50 → 0.57, 100 → 0.67, 1000+ → 1.0
func normalizeStars(stars int) float64 {
	if stars <= 1 {
		return 0.0
	}
	v := math.Log10(float64(stars)) / 3.0 // log10(1000) = 3
	if v > 1.0 {
		return 1.0
	}
	return v
}

func sourceQualityScore(r SearchResult) float64 {
	if isPreferredSkillRepo(r.Owner, r.Repo) {
		return 1.0
	}

	score := 0.15 // low base: unknown repos must earn their score
	path := strings.ToLower(strings.TrimSpace(r.Path))
	repo := strings.ToLower(strings.TrimSpace(r.Repo))

	if path == "skills/"+strings.ToLower(r.Name) || strings.HasPrefix(path, "skills/") {
		score += 0.25
	}
	if strings.Contains(repo, "skill") || strings.Contains(path, "/skills/") || strings.HasSuffix(path, "/skills") {
		score += 0.15
	}
	if pathLooksLikeSecondaryCopy(path) {
		score -= 0.25
	}

	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func repositoryMatchScore(r SearchResult, query string) float64 {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 0
	}

	repo := strings.ToLower(strings.TrimSpace(r.Repo))
	owner := strings.ToLower(strings.TrimSpace(r.Owner))
	fullName := strings.Trim(owner+"/"+repo, "/")

	if repo == query || fullName == query {
		return 1.0
	}
	if strings.Contains(repo, query) {
		return 0.7
	}

	return 0
}

func isPreferredSkillRepo(owner, repo string) bool {
	fullName := strings.ToLower(strings.TrimSpace(owner + "/" + repo))
	for _, preferred := range preferredSkillRepos {
		if fullName == strings.ToLower(preferred) {
			return true
		}
	}
	return false
}

func pathLooksLikeSecondaryCopy(path string) bool {
	if path == "" || path == "." {
		return false
	}
	segments := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	for _, segment := range segments {
		switch segment {
		case "docs", "doc", "example", "examples", "demo", "demos", "test", "tests", "tmp", "temp", "trash", "archive", "archives", "backup", "backups":
			return true
		}
	}
	return false
}
