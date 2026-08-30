package githook

import (
	"errors"
	"strings"
)

// ErrUsage is returned when git-hook is invoked with no hook name or an unknown one.
var ErrUsage = errors.New("usage: dots git-hook {reference-transaction|pre-commit|pre-rebase} [args]")

const (
	msgLeaveDefault   = "error: agent harness cannot leave the default branch on the primary checkout."
	hintLeaveDefault  = "  Stay on the default branch here. `git fetch`, `git pull origin main`, and `git reset --hard origin/<branch>` are allowed. Do other git work in a linked worktree."
	msgPrimaryStash   = "error: agent harness cannot update the stash on the primary checkout."
	hintPrimaryStash  = "  `git stash list` and `git stash show` are allowed here. Do other git work in a linked worktree."
	msgPrimaryWrite   = "error: agent harness cannot commit or rebase on the primary checkout."
	hintPrimaryWrite  = "  Do local git work in a linked worktree."
	msgLinkedDefault  = "error: agent harness cannot check out the default branch in a linked worktree."
	hintLinkedDefault = "  Use a feature branch in this worktree."
)

// RefusalError is a policy block. Git should print the message and exit 1.
type RefusalError struct {
	lines []string
}

// Error returns the refusal message and hint joined by newlines.
func (r *RefusalError) Error() string {
	if r == nil {
		return ""
	}
	return strings.Join(r.lines, "\n")
}

func refuse(message string, hint string) *RefusalError {
	return &RefusalError{lines: []string{message, hint}}
}
