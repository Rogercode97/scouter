package store

import (
	"errors"
	"fmt"

	driversqlite "modernc.org/sqlite"
)

// annotateSQLError ports Engram's driver-fence contract: a SQLite failure
// names the operation that ran and the numeric result code, keeps the %w
// chain so callers can errors.As the driver error, and never renders SQL
// text, bound arguments, DSN or filesystem paths. Non-driver errors pass
// through with the operation prefix only.
func annotateSQLError(op string, err error) error {
	if err == nil {
		return nil
	}
	var serr *driversqlite.Error
	if errors.As(err, &serr) {
		name, ok := driversqlite.ErrorCodeString[serr.Code()]
		if !ok {
			return fmt.Errorf("%s (code %d): %w", op, serr.Code(), err)
		}
		return fmt.Errorf("%s %s (code %d): %w", op, name, serr.Code(), err)
	}
	return fmt.Errorf("%s: %w", op, err)
}
