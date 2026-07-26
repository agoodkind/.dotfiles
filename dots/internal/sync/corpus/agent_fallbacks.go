package corpus

import (
	"fmt"
	"sort"
	"strings"

	"goodkind.io/.dotfiles/internal/sync/compilation"
)

const agentFallbackPrefix = "agent-"

func withAgentFallbackSkills(sourceSet compilation.CorpusSourceSet) (compilation.CorpusSourceSet, error) {
	skills := make(map[string]compilation.SkillSource, len(sourceSet.Skills)+len(sourceSet.Agents))
	for name, skill := range sourceSet.Skills {
		skills[name] = skill
	}

	agentNames := make([]string, 0, len(sourceSet.Agents))
	for name := range sourceSet.Agents {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)

	for _, agentName := range agentNames {
		skillName := agentFallbackPrefix + agentName
		if _, exists := skills[skillName]; exists {
			return sourceSet, fmt.Errorf("duplicate skill %q", skillName)
		}
		agent := sourceSet.Agents[agentName]
		skills[skillName] = compilation.SkillSource{
			Name: skillName,
			Files: map[string]string{
				"SKILL.md": renderAgentFallbackSkill(skillName, agent),
			},
		}
	}

	sourceSet.Skills = skills
	return sourceSet, nil
}

func renderAgentFallbackSkill(skillName string, agent compilation.AgentSource) string {
	var builder strings.Builder
	builder.WriteString("---\nname: ")
	builder.WriteString(skillName)
	builder.WriteString("\ndescription: Apply the ")
	builder.WriteString(agent.Name)
	builder.WriteString(" agent contract without subagent context isolation.\n---\n\n")
	builder.WriteString("# ")
	builder.WriteString(agent.Name)
	builder.WriteString(" agent fallback\n\n")
	builder.WriteString("This skill applies the `")
	builder.WriteString(agent.Name)
	builder.WriteString("` agent contract in the current conversation. This harness does not provide prompt-bearing subagent isolation, so this skill does not create a separate context.\n\n")
	if agent.ReadOnly {
		builder.WriteString("This fallback is read-only. Use only tools that inspect files, search content, or retrieve evidence. Do not edit files, run mutating commands, or change external state.\n\n")
	}
	builder.WriteString("## Prompt contract\n\n")
	builder.WriteString(strings.TrimSpace(agent.Body))
	builder.WriteString("\n")
	return builder.String()
}
