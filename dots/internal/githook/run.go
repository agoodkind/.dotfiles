package githook

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type hookName string

const (
	hookReferenceTransaction hookName = "reference-transaction"
	hookPreCommit            hookName = "pre-commit"
	hookPreRebase            hookName = "pre-rebase"
)

// Run applies agent git-hook policy for one git hook invocation.
func Run(ctx context.Context, args []string, stdin io.Reader, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, ErrUsage.Error())
		return ErrUsage
	}
	var err error
	switch hookName(args[0]) {
	case hookReferenceTransaction:
		state := ""
		if len(args) > 1 {
			state = args[1]
		}
		err = applyReferenceTransaction(ctx, state, stdin)
	case hookPreCommit, hookPreRebase:
		err = applyPrimaryWrite(ctx)
	default:
		fmt.Fprintf(stderr, "unknown git-hook %q\n", args[0])
		return ErrUsage
	}
	return writeHookError(stderr, err)
}

func writeHookError(stderr io.Writer, err error) error {
	if err == nil {
		return nil
	}
	var refusal *RefusalError
	if errors.As(err, &refusal) {
		fmt.Fprintln(stderr, refusal.Error())
		return err
	}
	return err
}
