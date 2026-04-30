// Package search provides workspace content search primitives for the agent's
// tooling layer. The package is dependency-free of tool/agent layers so that
// it can be tested directly against on-disk fixtures.
//
// Content searches files under a root directory for a regex pattern, applying
// a workspace-boundary check, a default skip list, and binary-file filtering.
package search

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DefaultSkipDirs is the standard list of directory base names that the search
// package skips when walking a workspace tree.
var DefaultSkipDirs = []string{".git", "node_modules", "vendor", "dist", "build", "target"}

// DefaultMaxFileSize is the default per-file size cap (1 MiB). Files larger
// than this are skipped silently.
const DefaultMaxFileSize int64 = 1 << 20

// DefaultHeadLimit is the default cap on Match entries returned by Content
// when Options.HeadLimit is zero.
const DefaultHeadLimit = 200

// binarySniffSize is the number of leading bytes inspected to classify a file
// as text or binary.
const binarySniffSize = 512

// Options configures a Content search.
type Options struct {
	// Pattern is the regular expression to match against each line. Required.
	Pattern string
	// Path optionally scopes the walk to a sub-path under root. Empty means
	// walk root directly. Both relative (joined to root) and absolute paths
	// are accepted, but the resolved path must remain under root.
	Path string
	// Glob optionally filters files by basename glob (filepath.Match syntax,
	// e.g. "*.go"). Empty means no filename filter.
	Glob string
	// CaseInsensitive enables case-insensitive matching.
	CaseInsensitive bool
	// HeadLimit caps the number of returned Match entries. Zero means use
	// DefaultHeadLimit. A negative value disables capping entirely.
	HeadLimit int
	// MaxFileSize skips files larger than this. Zero means use DefaultMaxFileSize.
	MaxFileSize int64
	// SkipDirs overrides the default skip-dir list. Nil means use DefaultSkipDirs.
	SkipDirs []string
}

// Match is a single content match: the file (workspace-relative, slash-separated),
// the 1-based line number, and the matched line's text.
type Match struct {
	Path string
	Line int
	Text string
}

// Result carries the matches plus truncation metadata.
//
// Total is the total number of matches found across the scan; len(Matches) is
// at most HeadLimit. Truncated is true when Total > len(Matches).
type Result struct {
	Matches   []Match
	Total     int
	Truncated bool
}

// Content walks root, opens text files matching the optional path/glob filters,
// and returns lines matching opts.Pattern. Results are sorted by (path, line)
// for deterministic ordering across calls.
func Content(ctx context.Context, root string, opts Options) (Result, error) {
	if opts.Pattern == "" {
		return Result{}, errors.New("search: pattern is required")
	}

	root = filepath.Clean(root)
	walkRoot := root
	if opts.Path != "" {
		sub, err := resolveUnderRoot(root, opts.Path)
		if err != nil {
			return Result{}, err
		}
		walkRoot = sub
	}

	re, err := compilePattern(opts.Pattern, opts.CaseInsensitive)
	if err != nil {
		return Result{}, err
	}

	skipSet := buildSkipSet(opts.SkipDirs)

	maxSize := opts.MaxFileSize
	if maxSize == 0 {
		maxSize = DefaultMaxFileSize
	}

	var matches []Match
	walkErr := filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, werr error) error {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if werr != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != walkRoot {
				if _, skip := skipSet[d.Name()]; skip {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		info, ierr := d.Info()
		if ierr != nil || info.Size() > maxSize {
			return nil
		}

		if opts.Glob != "" {
			ok, _ := filepath.Match(opts.Glob, d.Name())
			if !ok {
				return nil
			}
		}

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)

		fileMatches, ferr := scanFile(path, re)
		if ferr != nil {
			return nil
		}
		for _, m := range fileMatches {
			m.Path = relSlash
			matches = append(matches, m)
		}
		return nil
	})
	if walkErr != nil {
		return Result{}, walkErr
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		return matches[i].Line < matches[j].Line
	})

	total := len(matches)
	head := opts.HeadLimit
	if head == 0 {
		head = DefaultHeadLimit
	}
	truncated := false
	if head > 0 && total > head {
		matches = matches[:head]
		truncated = true
	}
	return Result{Matches: matches, Total: total, Truncated: truncated}, nil
}

// resolveUnderRoot normalizes p (relative to root if not absolute) and refuses
// any result that escapes the root tree.
func resolveUnderRoot(root, p string) (string, error) {
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", fmt.Errorf("search: path %q: %w", p, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("search: path %q is outside workspace root", p)
	}
	return abs, nil
}

func compilePattern(pat string, caseInsensitive bool) (*regexp.Regexp, error) {
	if caseInsensitive {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("search: invalid pattern: %w", err)
	}
	return re, nil
}

func buildSkipSet(override []string) map[string]struct{} {
	src := override
	if src == nil {
		src = DefaultSkipDirs
	}
	out := make(map[string]struct{}, len(src))
	for _, d := range src {
		out[d] = struct{}{}
	}
	return out
}

// scanFile sniffs the leading bytes for a NUL byte (binary heuristic), and
// otherwise streams the file line-by-line applying re. Returns matches with
// 1-based line numbers and Path left empty (caller fills in).
func scanFile(path string, re *regexp.Regexp) ([]Match, error) {
	f, err := os.Open(path) // #nosec G304 -- path comes from a bounded WalkDir under workspace root
	if err != nil {
		return nil, err
	}
	defer f.Close()

	head := make([]byte, binarySniffSize)
	n, _ := io.ReadFull(f, head)
	if n > 0 && bytes.IndexByte(head[:n], 0) >= 0 {
		return nil, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var matches []Match
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if re.MatchString(text) {
			matches = append(matches, Match{Line: line, Text: text})
		}
	}
	return matches, nil
}
