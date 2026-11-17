package scheduler

import (
	"context"
	"errors"
)

var ErrExperienceErasureDeferred = errors.New("experience erasure requires complete scoped authority and stopped-run proof")

// ReconcileExperienceErasure is a bounded maintenance pass, not a preview API.
// Its namespace-scoped fair cursor survives passes; it is not deletion authority.
func (b Backend) ReconcileExperienceErasure(ctx context.Context) error {
	value, _ := experienceErasureCursors.LoadOrStore(b.Namespace(), &experienceErasureCursor{runs: map[string]experienceErasurePosition{}})
	cursor := value.(*experienceErasureCursor)
	if !cursor.TryLock() {
		return ErrExperienceErasureDeferred
	}
	defer cursor.Unlock()
	return cursor.pass(ctx, b)
}
