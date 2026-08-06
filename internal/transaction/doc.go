// Package transaction implements the locked backup-verify-apply-rollback
// sequence (spec section 15.2) used by every mutating peer/interface
// operation: acquire lock, re-read fingerprint, backup, write to a temp
// file on the same filesystem, validate with wg tooling, atomically
// replace, apply with wg syncconf, verify, and roll back on any failure.
package transaction
