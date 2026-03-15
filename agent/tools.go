package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wildgecu/homer"
	"wildgecu/provider/tool"
	"wildgecu/skill"
)

// GetTimeInput is the input for the get_current_time tool.
type GetTimeInput struct {
	Timezone string `json:"timezone,omitempty" description:"IANA timezone name"`
}

// GetTimeOutput is the output for the get_current_time tool.
type GetTimeOutput struct {
	Time     string `json:"time"`
	Timezone string `json:"timezone"`
}

var getCurrentTimeTool = tool.NewTool("get_current_time", "Get the current time in a given timezone",
	func(ctx context.Context, in GetTimeInput) (GetTimeOutput, error) {
		tz := in.Timezone
		if tz == "" {
			tz = "UTC"
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return GetTimeOutput{}, fmt.Errorf("%w", err)
		}
		now := time.Now().In(loc)
		return GetTimeOutput{
			Time:     now.Format(time.RFC3339),
			Timezone: tz,
		}, nil
	},
)

// LoadSkillInput is the input for the load_skill tool.
type LoadSkillInput struct {
	Action string `json:"action" description:"Action to perform: 'list' to list available skills, 'load' to load a specific skill"`
	Name   string `json:"name,omitempty" description:"Name of the skill to load (required when action is 'load')"`
}

// LoadSkillOutput is the output for the load_skill tool.
type LoadSkillOutput struct {
	Skills  []SkillSummary `json:"skills,omitempty"`
	Name    string         `json:"name,omitempty"`
	Content string         `json:"content,omitempty"`
}

// SkillSummary is a brief summary of a skill for listing.
type SkillSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

func newLoadSkillTool(skillsHome homer.Homer) tool.Tool {
	return tool.NewTool("load_skill", "List and load domain-specific skills. Use action='list' to see available skills, action='load' with name to load a specific skill's content.",
		func(ctx context.Context, in LoadSkillInput) (LoadSkillOutput, error) {
			switch in.Action {
			case "list":
				skills, _ := skill.LoadAll(skillsHome)
				summaries := make([]SkillSummary, 0, len(skills))
				for _, s := range skills {
					summaries = append(summaries, SkillSummary{
						Name:        s.Name,
						Description: s.Description,
						Tags:        s.Tags,
					})
				}
				return LoadSkillOutput{Skills: summaries}, nil

			case "load":
				if strings.TrimSpace(in.Name) == "" {
					return LoadSkillOutput{}, fmt.Errorf("name is required when action is 'load'")
				}
				s, err := skill.Load(skillsHome, in.Name)
				if err != nil {
					return LoadSkillOutput{}, fmt.Errorf("loading skill %q: %w", in.Name, err)
				}
				return LoadSkillOutput{
					Name:    s.Name,
					Content: s.Content,
				}, nil

			default:
				return LoadSkillOutput{}, fmt.Errorf("unknown action %q: use 'list' or 'load'", in.Action)
			}
		},
	)
}
