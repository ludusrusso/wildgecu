package tools

import (
	"context"
	"fmt"
	"sort"

	"github.com/ludusrusso/wildgecu/pkg/provider/tool"
	"github.com/ludusrusso/wildgecu/pkg/search"
)

// SearchConfig configures the search tools (currently grep). Zero values fall
// back to the search package defaults.
type SearchConfig struct {
	// MaxResults caps the number of returned items per call (lines for
	// content mode, files for files_with_matches/count). Zero = use
	// search.DefaultHeadLimit.
	MaxResults int
	// MaxFileSizeBytes skips files larger than this. Zero = use
	// search.DefaultMaxFileSize.
	MaxFileSizeBytes int64
}

const (
	grepModeContent          = "content"
	grepModeFilesWithMatches = "files_with_matches"
	grepModeCount            = "count"
)

// SearchTools returns search tools (grep) bound to workDir.
func SearchTools(workDir string, cfg SearchConfig) []tool.Tool {
	return []tool.Tool{newGrepTool(workDir, cfg)}
}

type grepInput struct {
	Pattern         string `json:"pattern" description:"Regex pattern to search for in file contents"`
	Path            string `json:"path,omitempty" description:"Subdirectory under the workspace to scope the search (defaults to workspace root)"`
	Glob            string `json:"glob,omitempty" description:"Filename glob filter (e.g. *.go). Matched against file basename."`
	CaseInsensitive bool   `json:"case_insensitive,omitempty" description:"Case-insensitive match"`
	OutputMode      string `json:"output_mode,omitempty" description:"content (default, returns path:line:text) | files_with_matches | count"`
	HeadLimit       int    `json:"head_limit,omitempty" description:"Cap on returned items. Default 200."`
}

type grepMatchOut struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type grepFileCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

type grepOutput struct {
	Mode      string          `json:"mode"`
	Matches   []grepMatchOut  `json:"matches,omitempty"`
	Files     []string        `json:"files,omitempty"`
	Counts    []grepFileCount `json:"counts,omitempty"`
	Total     int             `json:"total"`
	Returned  int             `json:"returned"`
	Truncated bool            `json:"truncated,omitempty"`
	Indicator string          `json:"indicator,omitempty"`
}

func newGrepTool(workDir string, cfg SearchConfig) tool.Tool {
	return tool.NewTool("grep",
		"Search file contents by regex across the workspace. Prefer this over bash grep/rg. "+
			"Supports content (path:line:text), files_with_matches, and count output modes.",
		func(ctx context.Context, in grepInput) (grepOutput, error) {
			mode := in.OutputMode
			if mode == "" {
				mode = grepModeContent
			}
			if mode != grepModeContent && mode != grepModeFilesWithMatches && mode != grepModeCount {
				return grepOutput{}, fmt.Errorf("invalid output_mode %q (want content|files_with_matches|count)", mode)
			}

			defaultHead := cfg.MaxResults
			if defaultHead <= 0 {
				defaultHead = search.DefaultHeadLimit
			}
			head := in.HeadLimit
			if head <= 0 {
				head = defaultHead
			}

			// For non-content modes we need every match to aggregate
			// correctly; the post-aggregation cap is applied by the
			// tool wrapper.
			searchHead := head
			if mode != grepModeContent {
				searchHead = -1
			}

			res, err := search.Content(ctx, workDir, search.Options{
				Pattern:         in.Pattern,
				Path:            in.Path,
				Glob:            in.Glob,
				CaseInsensitive: in.CaseInsensitive,
				HeadLimit:       searchHead,
				MaxFileSize:     cfg.MaxFileSizeBytes,
			})
			if err != nil {
				return grepOutput{}, err
			}

			switch mode {
			case grepModeContent:
				return buildContentOutput(res), nil
			case grepModeFilesWithMatches:
				return buildFilesOutput(res, head), nil
			case grepModeCount:
				return buildCountOutput(res, head), nil
			}
			return grepOutput{}, fmt.Errorf("unreachable")
		},
	)
}

func buildContentOutput(res search.Result) grepOutput {
	out := grepOutput{Mode: grepModeContent, Total: res.Total}
	out.Matches = make([]grepMatchOut, 0, len(res.Matches))
	for _, m := range res.Matches {
		out.Matches = append(out.Matches, grepMatchOut{Path: m.Path, Line: m.Line, Text: m.Text})
	}
	out.Returned = len(out.Matches)
	if res.Truncated {
		out.Truncated = true
		out.Indicator = fmt.Sprintf("showing first %d of %d matches", out.Returned, res.Total)
	}
	return out
}

func buildFilesOutput(res search.Result, head int) grepOutput {
	seen := map[string]struct{}{}
	files := make([]string, 0)
	for _, m := range res.Matches {
		if _, ok := seen[m.Path]; ok {
			continue
		}
		seen[m.Path] = struct{}{}
		files = append(files, m.Path)
	}
	sort.Strings(files)
	totalFiles := len(files)
	truncated := false
	if head > 0 && totalFiles > head {
		files = files[:head]
		truncated = true
	}
	out := grepOutput{
		Mode:     grepModeFilesWithMatches,
		Files:    files,
		Total:    totalFiles,
		Returned: len(files),
	}
	if truncated {
		out.Truncated = true
		out.Indicator = fmt.Sprintf("showing first %d of %d files", out.Returned, totalFiles)
	}
	return out
}

func buildCountOutput(res search.Result, head int) grepOutput {
	counts := map[string]int{}
	for _, m := range res.Matches {
		counts[m.Path]++
	}
	rows := make([]grepFileCount, 0, len(counts))
	for p, c := range counts {
		rows = append(rows, grepFileCount{Path: p, Count: c})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Path < rows[j].Path
	})
	totalFiles := len(rows)
	truncated := false
	if head > 0 && totalFiles > head {
		rows = rows[:head]
		truncated = true
	}
	out := grepOutput{
		Mode:     grepModeCount,
		Counts:   rows,
		Total:    totalFiles,
		Returned: len(rows),
	}
	if truncated {
		out.Truncated = true
		out.Indicator = fmt.Sprintf("showing first %d of %d files", out.Returned, totalFiles)
	}
	return out
}
