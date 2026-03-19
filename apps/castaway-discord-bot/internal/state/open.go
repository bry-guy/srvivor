package state

import (
	"context"
	"fmt"
)

func Open(ctx context.Context, opts Options) (Store, error) {
	switch opts.Backend {
	case BackendBolt, "":
		if opts.BoltPath == "" {
			return nil, fmt.Errorf("bolt state path is required")
		}
		return OpenBolt(opts.BoltPath)
	case BackendPostgres:
		return OpenPostgres(ctx, opts.PostgresURL, opts.AutoMigrate)
	default:
		return nil, fmt.Errorf("unsupported state backend: %s", opts.Backend)
	}
}
