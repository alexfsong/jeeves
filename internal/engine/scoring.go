package engine

import (
	"math"
	"net/url"
	"strings"

	"github.com/alexfsong/jeeves/internal/source"
	"github.com/alexfsong/jeeves/internal/store"
)

// ScoreTFIDF scores results based on term frequency in the query.
func ScoreTFIDF(results []source.Result, query string) {
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return
	}

	for i := range results {
		text := strings.ToLower(results[i].Title + " " + results[i].Snippet)
		docTerms := tokenize(text)
		if len(docTerms) == 0 {
			continue
		}

		// Term frequency
		termFreq := make(map[string]int)
		for _, t := range docTerms {
			termFreq[t]++
		}

		var score float64
		for _, qt := range queryTerms {
			tf := float64(termFreq[qt]) / float64(len(docTerms))
			if tf > 0 {
				score += tf
			}
		}

		// Normalize
		score = score / float64(len(queryTerms))

		// Blend with position-based score (keep some of the original ordering)
		results[i].Score = 0.6*score + 0.4*results[i].Score
	}
}

// RankResults sorts results by score descending and deduplicates by URL.
func RankResults(results []source.Result) []source.Result {
	// Deduplicate by URL
	seen := make(map[string]bool)
	var deduped []source.Result
	for _, r := range results {
		if r.URL != "" && seen[r.URL] {
			continue
		}
		if r.URL != "" {
			seen[r.URL] = true
		}
		deduped = append(deduped, r)
	}

	// Sort by score (insertion sort — small lists)
	for i := 1; i < len(deduped); i++ {
		for j := i; j > 0 && deduped[j].Score > deduped[j-1].Score; j-- {
			deduped[j], deduped[j-1] = deduped[j-1], deduped[j]
		}
	}

	return deduped
}

// ApplyTrustBoost adjusts result scores based on trusted source domain matches.
// Per-topic sources override global ones for the same domain.
func ApplyTrustBoost(results []source.Result, trusted []store.TrustedSource) {
	if len(trusted) == 0 {
		return
	}

	// Build domain → trust level map. Topic-specific entries are appended after
	// global ones, so they naturally override in the map.
	trustMap := make(map[string]float64)
	for _, ts := range trusted {
		trustMap[ts.Domain] = ts.TrustLevel
	}

	for i := range results {
		domain := extractDomain(results[i].URL)
		if domain == "" {
			continue
		}
		if boost, ok := trustMap[domain]; ok {
			results[i].Score = clamp(results[i].Score*boost, 0, 1)
		}
	}
}

func extractDomain(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	// Strip www. prefix for consistent matching
	host = strings.TrimPrefix(host, "www.")
	return host
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	words := strings.Fields(s)
	var tokens []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}—–-")
		if len(w) > 1 && !isStopWord(w) {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "has": true, "have": true, "had": true, "do": true,
	"does": true, "did": true, "will": true, "would": true, "could": true,
	"should": true, "may": true, "might": true, "shall": true, "can": true,
	"this": true, "that": true, "these": true, "those": true, "it": true,
	"its": true, "not": true, "no": true, "what": true, "how": true, "why": true,
	"when": true, "where": true, "who": true, "which": true, "from": true,
}

func isStopWord(w string) bool {
	return stopWords[w]
}

// used only for stable-ish scoring normalization
func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

var _ = clamp // suppress unused
