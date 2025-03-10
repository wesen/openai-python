package realtime

import (
	"context"
)

// component interface for all components of the client
type component interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
