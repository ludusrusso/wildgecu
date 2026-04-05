package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"wildgecu/pkg/provider/tool"
)

// FileTools returns file-operation tools bound to workDir.
func FileTools(workDir string) []tool.Tool {
	return []tool.Tool{
		newListFilesTool(workDir),
		newReadFileTool(workDir),
		newWriteFileTool(workDir),
		newUpdateFileTool(workDir),
	}
}

// resolvePath resolves inputPath relative to workDir.
func resolvePath(workDir, inputPath string) string {
	if filepath.IsAbs(inputPath) {
		return filepath.Clean(inputPath)
	}
	return filepath.Join(workDir, inputPath)
}

// --- list_files ---

type listFilesInput struct {
	Path    string `json:"path,omitempty" description:"Directory to list (defaults to working directory)"`
	Pattern string `json:"pattern,omitempty" description:"Glob pattern to filter entries (e.g. *.go)"`
}

type fileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type listFilesOutput struct {
	Path    string      `json:"path"`
	Entries []fileEntry `json:"entries"`
}

func newListFilesTool(workDir string) tool.Tool {
	return tool.NewTool("list_files", "List files and directories. Use this instead of bash ls.",
		func(ctx context.Context, in listFilesInput) (listFilesOutput, error) {
			dir := workDir
			if in.Path != "" {
				dir = resolvePath(workDir, in.Path)
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				return listFilesOutput{}, fmt.Errorf("reading directory %s: %w", dir, err)
			}

			result := make([]fileEntry, 0, len(entries))
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
				result = append(result, fileEntry{
					Name:  e.Name(),
					IsDir: e.IsDir(),
					Size:  info.Size(),
				})
				if len(result) >= 500 {
					break
				}
			}

			return listFilesOutput{Path: dir, Entries: result}, nil
		},
	)
}

// --- read_file ---

type readFileInput struct {
	Path   string `json:"path" description:"File path to read"`
	Offset int    `json:"offset,omitempty" description:"1-based starting line number"`
	Limit  int    `json:"limit,omitempty" description:"Maximum number of lines to return"`
}

type readFileOutput struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	TotalLines int    `json:"total_lines"`
}

func newReadFileTool(workDir string) tool.Tool {
	return tool.NewTool("read_file", "Read a file's content with line numbers. Use this instead of bash cat/head/tail.",
		func(ctx context.Context, in readFileInput) (readFileOutput, error) {
			p := resolvePath(workDir, in.Path)

			data, err := os.ReadFile(p)
			if err != nil {
				return readFileOutput{}, fmt.Errorf("reading %s: %w", p, err)
			}

			if !utf8.Valid(data) {
				return readFileOutput{}, fmt.Errorf("file %s appears to be binary", p)
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

			return readFileOutput{
				Path:       p,
				Content:    b.String(),
				TotalLines: totalLines,
			}, nil
		},
	)
}

// --- write_file ---

type writeFileInput struct {
	Path    string `json:"path" description:"File path to write"`
	Content string `json:"content" description:"Full file content to write"`
	Create  bool   `json:"create,omitempty" description:"Create parent directories if they don't exist"`
}

type writeFileOutput struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

func newWriteFileTool(workDir string) tool.Tool {
	return tool.NewTool("write_file", "Write content to a file. Always read_file first to understand context.",
		func(ctx context.Context, in writeFileInput) (writeFileOutput, error) {
			p := resolvePath(workDir, in.Path)

			if in.Create {
				dir := filepath.Dir(p)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return writeFileOutput{}, fmt.Errorf("creating directories for %s: %w", p, err)
				}
			}

			content := []byte(in.Content)
			if err := os.WriteFile(p, content, 0o644); err != nil {
				return writeFileOutput{}, fmt.Errorf("writing %s: %w", p, err)
			}

			return writeFileOutput{Path: p, Bytes: len(content)}, nil
		},
	)
}

// --- update_file ---

type updateFileInput struct {
	Path      string `json:"path" description:"File path to update"`
	OldString string `json:"old_string" description:"Exact string to find in the file (must be unique)"`
	NewString string `json:"new_string" description:"Replacement string"`
}

type updateFileOutput struct {
	Path string `json:"path"`
}

func newUpdateFileTool(workDir string) tool.Tool {
	return tool.NewTool("update_file", "Replace an exact string in a file. The old_string must appear exactly once. Always read_file first.",
		func(ctx context.Context, in updateFileInput) (updateFileOutput, error) {
			p := resolvePath(workDir, in.Path)

			data, err := os.ReadFile(p)
			if err != nil {
				return updateFileOutput{}, fmt.Errorf("reading %s: %w", p, err)
			}

			content := string(data)
			count := strings.Count(content, in.OldString)
			if count == 0 {
				return updateFileOutput{}, fmt.Errorf("old_string not found in %s", p)
			}
			if count > 1 {
				return updateFileOutput{}, fmt.Errorf("old_string appears %d times in %s — must be unique", count, p)
			}

			updated := strings.Replace(content, in.OldString, in.NewString, 1)
			if err := os.WriteFile(p, []byte(updated), 0o644); err != nil {
				return updateFileOutput{}, fmt.Errorf("writing %s: %w", p, err)
			}

			return updateFileOutput{Path: p}, nil
		},
	)
}
