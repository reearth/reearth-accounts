package workspaceuc

import "errors"

// isTransactionConflict reports whether err is the database aborting the
// transaction because it lost a race with another one, so the same request can
// succeed when retried. It matches on the drivers' error methods rather than
// their types so the use case does not import them:
//   - pgx's *pgconn.PgError.SQLState: 40001 (serialization_failure) and 40P01
//     (deadlock_detected);
//   - the Mongo driver's HasErrorLabel: TransientTransactionError, which is
//     what a write conflict at commit carries (reearthx mongox maps it to
//     usecasex.ErrTransaction only for errors raised inside the transaction,
//     not for the one Tx.End returns).
//
// TestTransferOwner_DatabaseConflictIsTransactionError pins both against the
// real driver error types.
func isTransactionConflict(err error) bool {
	var pg interface{ SQLState() string }
	if errors.As(err, &pg) {
		switch pg.SQLState() {
		case "40001", "40P01":
			return true
		}
	}
	var mg interface{ HasErrorLabel(string) bool }
	return errors.As(err, &mg) && mg.HasErrorLabel("TransientTransactionError")
}
