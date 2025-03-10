package realtime

import (
	"fmt"
)

// Common error definitions used across the package
var (
	ErrAlreadyRunning = fmt.Errorf("component is already running")
	ErrNotRunning     = fmt.Errorf("component is not running")
	ErrNotInitialized = fmt.Errorf("component is not initialized")
	ErrInvalidState   = fmt.Errorf("component is in an invalid state")
)
