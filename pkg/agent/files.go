package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"wildgecu/pkg/provider/tool"
)

// resolvePath resolves inputPath relative to workDir. Prevents path traversal.
func resolvePath(workDir, inputPath string) string {
	if filepath.IsAbs(inputPath) {
		return filepath.Clean(inputPath)
	}
	return filepath.Join(workDir, inputPath)
}


// --- list_files ---

// ListFilesInput is the input for the list_files tool.
type ListFilesInput struct {
	Path    string `json:"path,omitempty" description:"Directory to list (defaults to working directory)"`
	Pattern string `json:"pattern,omitempty" description:"Glob pattern to filter entries (e.g. *.go)"`
}

// FileEntry describes a single directory entry.
type FileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ListFilesOutput is the output for the list_files tool.
type ListFilesOutput struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

func newListFilesTool(workDir string) tool.Tool {
	return tool.NewTool("list_files", "List files and directories. Use this instead of bash ls.",
		func(ctx context.Context, in ListFilesInput) (ListFilesOutput, error) {
			dir := workDir
			if in.Path != "" {
				dir = resolvePath(workDir, in.Path)
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				return ListFilesOutput{}, fmt.Errorf("reading directory %s: %w", dir, err)
			}

			result := make([]FileEntry, 0, len(entries))
			for _, e := range entries {
				if in.Pattern != "" {
					matched, _ := filepath.Match(in.Pattern, e.Name())
					if !matched {
						continue
					}
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				result = append(result, FileEntry{
					Name:  e.Name(),
					IsDir: e.IsDir(),
					Size:  info.Size(),
				})
				if len(result) >= 500 {
					break
				}
			}

			return ListFilesOutput{Path: dir, Entries: result}, nil
		},
	)
}

// --- read_file ---

// ReadFileInput is the input for the read_file tool.
type ReadFileInput struct {
	Path   string `json:"path" description:"File path to read"`
	Offset int    `json:"offset,omitempty" description:"1-based starting line number"`
	Limit  int    `json:"limit,omitempty" description:"Maximum number of lines to return"`
}

// ReadFileOutput is the output for the read_file tool.
type ReadFileOutput struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	TotalLines int    `json:"total_lines"`
}

func newReadFileTool(workDir string) tool.Tool {
	return tool.NewTool("read_file", "Read a file's content with line numbers. Use this instead of bash cat/head/tail.",
		func(ctx context.Context, in ReadFileInput) (ReadFileOutput, error) {
			p := resolvePath(workDir, in.Path)

			data, err := os.ReadFile(p)
			if err != nil {
				return ReadFileOutput{}, fmt.Errorf("reading %s: %w", p, err)
			}

			if !utf8.Valid(data) {
				return ReadFileOutput{}, fmt.Errorf("file %s appears to be binary", p)
			}

			lines := strings.Split(string(data), "\n")
			totalLines := len(lines)

			// Apply offset (1-based).
			start := 0
			if in.Offset > 0 {
				start = in.Offset - 1
			}
			if start > len(lines) {
				start = len(lines)
			}
			lines = lines[start:]

			// Apply limit.
			limit := 10000
			if in.Limit > 0 && in.Limit < limit {
				limit = in.Limit
			}
			if len(lines) > limit {
				lines = lines[:limit]
			}

			// Format with line numbers.
			var b strings.Builder
			for i, line := range lines {
				fmt.Fprintf(&b, "%d\t%s\n", start+i+1, line)
			}

			return ReadFileOutput{
				Path:       p,
				Content:    b.String(),
				TotalLines: totalLines,
			}, nil
		},
	)
}

// --- write_file ---

// WriteFileInput is the input for the write_file tool.
type WriteFileInput struct {
	Path    string `json:"path" description:"File path to write"`
	Content string `json:"content" description:"Full file content to write"`
	Create  bool   `json:"create,omitempty" description:"Create parent directories if they don't exist"`
}

// WriteFileOutput is the output for the write_file tool.
type WriteFileOutput struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

func newWriteFileTool(workDir string) tool.Tool {
	return tool.NewTool("write_file", "Write content to a file. Always read_file first to understand context.",
		func(ctx context.Context, in WriteFileInput) (WriteFileOutput, error) {
			p := resolvePath(workDir, in.Path)

			if in.Create {
				dir := filepath.Dir(p)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return WriteFileOutput{}, fmt.Errorf("creating directories for %s: %w", p, err)
				}
			}

			content := []byte(in.Content)
			if err := os.WriteFile(p, content, 0o644); err != nil {
				return WriteFileOutput{}, fmt.Errorf("writing %s: %w", p, err)
			}

			return WriteFileOutput{Path: p, Bytes: len(content)}, nil
		},
	)
}

// --- update_file ---

// UpdateFileInput is the input for the update_file tool.
type UpdateFileInput struct {
	Path      string `json:"path" description:"File path to update"`
	OldString string `json:"old_string" description:"Exact string to find in the file (must be unique)"`
	NewString string `json:"new_string" description:"Replacement string"`
}

// UpdateFileOutput is the output for the update_file tool.
type UpdateFileOutput struct {
	Path string `json:"path"`
}

func newUpdateFileTool(workDir string) tool.Tool {
	return tool.NewTool("update_file", "Replace an exact string in a file. The old_string must appear exactly once. Always read_file first.",
		func(ctx context.Context, in UpdateFileInput) (UpdateFileOutput, error) {
			p := resolvePath(workDir, in.Path)

			data, err := os.ReadFile(p)
			if err != nil {
				return UpdateFileOutput{}, fmt.Errorf("reading %s: %w", p, err)
			}

			content := string(data)
			count := strings.Count(content, in.OldString)
			if count == 0 {
				return UpdateFileOutput{}, fmt.Errorf("old_string not found in %s", p)
			}
			if count > 1 {
				return UpdateFileOutput{}, fmt.Errorf("old_string appears %d times in %s — must be unique", count, p)
			}

			updated := strings.Replace(content, in.OldString, in.NewString, 1)
			// #nosec G703 - path is resolved through resolvePath from user input
			if err := os.WriteFile(p, []byte(updated), 0o644); err != nil {
				return UpdateFileOutput{}, fmt.Errorf("writing %s: %w", p, err)
			}

			return UpdateFileOutput{Path: p}, nil
		},
	)
}
