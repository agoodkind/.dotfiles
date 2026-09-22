package githook

import (
	"context"
	"io"
)

const preparingState = "preparing"

func applyReferenceTransaction(ctx context.Context, state string, stdin io.Reader) error {
	if state != preparingState {
		return nil
	}
	if !harnessActive() {
		return nil
	}
	layout := inspectRepo(ctx)
	if !layout.ready {
		return nil
	}
	updates, err := readRefUpdates(ctx, stdin)
	if err != nil {
		return err
	}
	return applyRefUpdates(ctx, layout, updates)
}

func applyRefUpdates(ctx context.Context, layout repoLayout, updates []refUpdate) error {
	headOld := ""
	headNew := ""
	for _, update := range updates {
		switch {
		case update.refName == headRef:
			headOld = update.oldValue
			headNew = update.newValue
		case isStashRef(update.refName):
			if layout.primary {
				return refuse(msgPrimaryStash, hintPrimaryStash)
			}
		}
	}
	if headNew == "" {
		return nil
	}
	return applyHeadUpdate(ctx, layout, headOld, headNew)
}

func applyHeadUpdate(ctx context.Context, layout repoLayout, headOld string, headNew string) error {
	target, symbolic := branchFromSymbolicValue(headNew)
	if symbolic {
		if layout.primary && worktreeHeadSetup(ctx) {
			return nil
		}
		return applySymbolicHead(ctx, layout, target)
	}
	if !isZeroOID(headOld) {
		return nil
	}
	if layout.primary && worktreeHeadSetup(ctx) {
		return nil
	}
	onDefault, err := currentBranchIsDefault(ctx)
	if err != nil {
		return err
	}
	if layout.primary && onDefault {
		if pinnedSubmoduleCheckout(ctx, headNew) {
			return nil
		}
		return refuse(msgLeaveDefault, hintLeaveDefault)
	}
	return nil
}

func applySymbolicHead(ctx context.Context, layout repoLayout, target string) error {
	targetIsDefault, err := isDefaultBranch(ctx, target)
	if err != nil {
		return err
	}
	if layout.primary {
		onDefault, defaultErr := currentBranchIsDefault(ctx)
		if defaultErr != nil {
			return defaultErr
		}
		if onDefault && !targetIsDefault {
			return refuse(msgLeaveDefault, hintLeaveDefault)
		}
		return nil
	}
	if targetIsDefault {
		return refuse(msgLinkedDefault, hintLinkedDefault)
	}
	return nil
}

func applyPrimaryWrite(ctx context.Context) error {
	if !harnessActive() {
		return nil
	}
	layout := inspectRepo(ctx)
	if !layout.ready {
		return nil
	}
	if layout.primary {
		return refuse(msgPrimaryWrite, hintPrimaryWrite)
	}
	return nil
}
