package suggest

import (
	"sort"
	"strings"
)

// levenshtein computes edit distance between two strings using two-row DP.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min3(del, ins, sub)
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

type scored struct {
	name  string
	score float64
}

// Suggest returns up to n species names closest to the input.
// Levenshtein distance is computed case-insensitively; substring matches get
// their distance halved as a relevance boost.
func Suggest(input string, species []string, n int) []string {
	lower := strings.ToLower(input)

	results := make([]scored, len(species))
	for i, s := range species {
		dist := float64(levenshtein(lower, strings.ToLower(s)))
		if strings.Contains(strings.ToLower(s), lower) {
			dist /= 2
		}
		results[i] = scored{name: s, score: dist}
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].score < results[j].score
	})

	if n > len(results) {
		n = len(results)
	}

	out := make([]string, n)
	for i := range out {
		out[i] = results[i].name
	}
	return out
}
