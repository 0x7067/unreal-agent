package filesearch

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/sahilm/fuzzy"
)

type Match struct {
	Path      string
	Positions []int
}

type filenameSource []string

func (paths filenameSource) Len() int { return len(paths) }

func (paths filenameSource) String(i int) string {
	return paths[i][strings.LastIndexByte(paths[i], '/')+1:]
}

type searchMatch struct {
	fuzzy.Match
	filenameRank int
}

func (index Index) Search(ctx context.Context, query string, limit int) ([]Match, error) {
	if limit <= 0 {
		return nil, nil
	}
	query = strings.TrimPrefix(query, "./")
	best := make([]searchMatch, 0, min(limit, len(index.files)))
	compare := func(a, b searchMatch) int {
		if order := cmp.Compare(b.filenameRank, a.filenameRank); order != 0 {
			return order
		}
		if order := cmp.Compare(b.Score, a.Score); order != 0 {
			return order
		}
		if order := cmp.Compare(len(a.Str), len(b.Str)); order != 0 {
			return order
		}
		return strings.Compare(a.Str, b.Str)
	}
	keep := func(result searchMatch) {
		position, _ := slices.BinarySearchFunc(best, result, compare)
		if position >= limit {
			return
		}
		best = slices.Insert(best, position, result)
		if len(best) > limit {
			best = best[:limit]
		}
	}
	const batchSize = 256
	for start := 0; start < len(index.files); start += batchSize {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		paths := index.files[start:min(start+batchSize, len(index.files))]
		if query == "" {
			for _, path := range paths {
				result := fuzzy.Match{Str: path, Score: -utf8.RuneCountInString(path)}
				keep(searchMatch{Match: result})
			}
			continue
		}
		var filenameMatches [batchSize]bool
		if !strings.ContainsRune(query, '/') {
			for _, result := range fuzzy.FindFromNoSort(query, filenameSource(paths)) {
				filenameMatches[result.Index] = true
				rank := 1
				if strings.EqualFold(query, result.Str) {
					rank++
				}
				path := paths[result.Index]
				offset := len(path) - len(result.Str)
				for i := range result.MatchedIndexes {
					result.MatchedIndexes[i] += offset
				}
				result.Str = path
				keep(searchMatch{Match: result, filenameRank: rank})
			}
		}
		for _, result := range fuzzy.FindNoSort(query, paths) {
			if !filenameMatches[result.Index] {
				keep(searchMatch{Match: result})
			}
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	matches := make([]Match, 0, len(best))
	for _, result := range best {
		positions := make([]int, len(result.MatchedIndexes))
		for i, byteOffset := range result.MatchedIndexes {
			positions[i] = utf8.RuneCountInString(result.Str[:byteOffset])
		}
		matches = append(matches, Match{Path: result.Str, Positions: positions})
	}
	return matches, nil
}
