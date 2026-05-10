// Package prefercache implements dispatch-side prefer-alias cache rebuilding.
package prefercache

import (
	"context"
	"fmt"
	"log/slog"

	baseprefercache "goodkind.io/.dotfiles/internal/prefercache"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// Rebuild triggers a prefer-alias cache rebuild via the base prefercache package.
func Rebuild(ctx context.Context, dotfiles string, force bool, dispatchLogger *telemetry.Logger) error {
	if err := baseprefercache.Rebuild(ctx, dotfiles, force, dispatchLogger); err != nil {
		slog.ErrorContext(ctx, "dispatch/prefercache: Rebuild failed", "err", err)
		return fmt.Errorf("rebuilding prefer cache: %w", err)
	}
	return nil
}
