package learning

import "testing"

func TestNewStoreHasEveryTrustBoundary(t *testing.T) {
	store := NewStore()
	if store.Authorize == nil || store.VerifySource == nil || store.VerifyClaim == nil ||
		store.VerifyBenefit == nil || store.VerifyIntent == nil {
		t.Fatal("learning store omitted a fail-closed trust boundary")
	}
}
