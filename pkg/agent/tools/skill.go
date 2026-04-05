package tools

import (
	"context"
	"fmt"
	"strings"

	"wildgecu/pkg/provider/tool"
	"wildgecu/pkg/skill"
)

// SkillTools returns the load_skill tool bound to skillsDir.
// Returns an empty slice if skillsDir is empty.
func SkillTools(skillsDir string) []tool.Tool {
	if skillsDir == "" {
		return nil
	}
	return []tool.Tool{newLoadSkillTool(skillsDir)}
}

// --- load_skill ---

type loadSkillInput struct {
	Action string `json:"action" description:"Action to perform: 'list' to list available skills, 'load' to load a specific skill"`
	Name   string `json:"name,omitempty" description:"Name of the skill to load (required when action is 'load')"`
}

type loadSkillOutput struct {
	Skills  []skillSummary `json:"skills,omitempty"`
	Name    string         `json:"name,omitempty"`
	Content string         `json:"content,omitempty"`
}

type skillSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

func newLoadSkillTool(skillsDir string) tool.Tool {
	return tool.NewTool("load_skill", "List and load domain-specific skills. Use action='list' to see available skills, action='load' with name to load a specific skill's content.",
		func(ctx context.Context, in loadSkillInput) (loadSkillOutput, error) {
			switch in.Action {
			case "list":
				skills, _ := skill.LoadAll(skillsDir)
				summaries := make([]skillSummary, 0, len(skills))
				for _, s := range skills {
					summaries = append(summaries, skillSummary{
						Name:        s.Name,
						Description: s.Description,
						Tags:        s.Tags,
					})
				}
				return loadSkillOutput{Skills: summaries}, nil

			case "load":
				if strings.TrimSpace(in.Name) == "" {
					return loadSkillOutput{}, fmt.Errorf("name is required when action is 'load'")
				}
				s, err := skill.Load(skillsDir, in.Name)
				if err != nil {
					return loadSkillOutput{}, fmt.Errorf("loading skill %q: %w", in.Name, err)
				}
				return loadSkillOutput{
					Name:    s.Name,
					Content: s.Content,
				}, nil

			default:
				return loadSkillOutput{}, fmt.Errorf("unknown action %q: use 'list' or 'load'", in.Action)
			}
		},
	)
}
