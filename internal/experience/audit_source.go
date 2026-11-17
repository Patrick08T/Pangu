package experience

import "errors"

func redactedUnproven(reason string) AuditState {
	state := unprovenAudit(reason)
	state.Redacted = true
	return state
}

func sourceAuditState(err error) (AuditState, error) {
	if err == nil {
		return validAudit(), nil
	}
	if errors.Is(err, ErrForbidden) {
		return restrictedAudit(), nil
	}
	if errors.Is(err, ErrNotFound) {
		return redactedUnproven("source_missing"), nil
	}
	if errors.Is(err, ErrInvalidated) {
		return AuditState{Status: "invalid", InvalidReason: "source_invalidated", Redacted: true}, nil
	}
	return AuditState{}, err
}
