package githook

import (
	"context"
	"errors"
	"io"
)

const (
	preparingState = "preparing"
	preparedState  = "prepared"
)

var errUnknownCheckout = errors.New("cannot identify the worktree for the proposed HEAD change")

func applyReferenceTransaction(ctx context.Context, state string, stdin io.Reader) error {
	if state != preparingState && state != preparedState {
		return nil
	}

	if state == preparingState && !harnessActive() {
		return nil
	}

	if state == preparedState && !checkoutHarnessActive() {
		return nil
	}

	updates, err := readRefUpdates(ctx, stdin)
	if err != nil {
		return err
	}

	for _, update := range updates {
		if state == preparingState && isStashRef(update.refName) {
			layout := inspectRepo(ctx)
			if layout.primary && primaryMutationsBlocked(ctx) {
				return refuse(msgPrimaryStash, hintPrimaryStash)
			}
		}

		if state == preparedState && update.refName == headRef {
			layout := inspectRepo(ctx)
			if !layout.ready {
				return errUnknownCheckout
			}

			if err := applyHeadUpdate(ctx, layout, update); err != nil {
				return err
			}
		}
	}

	return nil
}

func applyHeadUpdate(ctx context.Context, layout repoLayout, update refUpdate) error {
	if worktreeHeadSetup(ctx) {
		return nil
	}

	target, symbolic := branchFromSymbolicValue(update.newValue)
	if symbolic {
		return applySymbolicHead(ctx, layout, target)
	}

	if !layout.primary || pinnedSubmoduleCheckout(ctx, update.newValue) {
		return nil
	}

	if update.oldValue == update.newValue && !isZeroOID(update.newValue) {
		if _, attached := currentBranch(ctx); !attached {
			return nil
		}
	}

	return refuse(msgLeaveDefault, hintLeaveDefault)
}

func applySymbolicHead(ctx context.Context, layout repoLayout, target string) error {
	defaultName, err := checkoutDefaultBranch(ctx)
	if err != nil {
		return err
	}

	if layout.primary && target != defaultName {
		return refuse(msgLeaveDefault, hintLeaveDefault)
	}

	if !layout.primary && target == defaultName {
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

	if layout.primary && primaryMutationsBlocked(ctx) {
		return refuse(msgPrimaryWrite, hintPrimaryWrite)
	}

	return nil
}
