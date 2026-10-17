//go:build !lite

package telemetry

import (
	_ "modernc.org/sqlite/vec"
	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

// DriverAvailable indicates whether the SQLite driver is compiled in.
const DriverAvailable = true
