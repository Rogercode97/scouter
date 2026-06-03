//go:build !lite

package telemetry

import (
	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	_ "github.com/ncruces/go-sqlite3/driver"
)

// DriverAvailable indicates whether the SQLite driver is compiled in.
const DriverAvailable = true
