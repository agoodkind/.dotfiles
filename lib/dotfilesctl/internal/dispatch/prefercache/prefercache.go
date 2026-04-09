package prefercache

import (
	"context"

	baseprefercache "github.com/agoodkind/.dotfiles/internal/prefercache"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

func Rebuild(ctx context.Context, dotfiles string, force bool, dispatchLogger *telemetry.Logger) error {
	return baseprefercache.Rebuild(ctx, dotfiles, force, dispatchLogger)
}
