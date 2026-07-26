package compilation

// CorpusSourceSet contains normalized rules, skills, and agents from every configured corpus source.
type CorpusSourceSet struct {
	Rules  map[string]RuleSource
	Skills map[string]SkillSource
	Agents map[string]AgentSource
}

// SkillSource contains one normalized skill and its relative file contents.
type SkillSource struct {
	Name  string
	Files map[string]string
}

// AgentSource contains one normalized agent contract and its permission mode.
type AgentSource struct {
	Name        string
	Description string
	Body        string
	ReadOnly    bool
}
