package series

import (
	"context"

	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// InitAndStart is a quick way to run Init and Start for a filecoin process, between calls
// to init and start, an optional function can be passed to perform any actions (generally
// configuration) prior to the daemon starting.
func InitAndStart(ctx context.Context, node *fast.Filecoin, fn func(*fast.Filecoin) error) error {
	if _, err := node.InitDaemon(ctx); err != nil {
		return err
	}

	if fn != nil {
		if err := fn(node); err != nil {
			return err
		}
	}

	if _, err := node.StartDaemon(ctx, true); err != nil {
		return err
	}

	return nil
}
