package githook

import "os"

// harnessActive reports whether the current process is a non-Cursor agent CLI.
// Names match home/.zshenv except CURSOR_AGENT, which is omitted so Cursor may
// do git work on the primary checkout. DOTFILES_AGENT_SHELL is ignored because
// Cursor sets it. There is no override environment variable.
func harnessActive() bool {
	if os.Getenv("CLAUDECODE") != "" {
		return true
	}
	if os.Getenv("CODEX_CI") != "" {
		return true
	}
	if os.Getenv("GEMINI_CLI") != "" {
		return true
	}
	return false
}
