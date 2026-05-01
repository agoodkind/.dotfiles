package prefercache

import (
	"context"

	baseprefercache "goodkind.io/.dotfiles/internal/prefercache"
	"goodkind.io/.dotfiles/internal/telemetry"
)

func Rebuild(ctx context.Context, dotfiles string, force bool, dispatchLogger *telemetry.Logger) error {
	return baseprefercache.Rebuild(ctx, dotfiles, force, dispatchLogger)
}
