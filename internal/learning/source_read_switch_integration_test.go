//go:build integration

package learning

import "testing"

func TestSourceReadSwitchRevokesEvenWhenLearningRemainsEnabled(t *testing.T) {
	f := newLearningFixture(t)
	version := f.canary(t)
	policy, err := f.store.Policy(f.ctx, f.scope)
	if err != nil {
		t.Fatal(err)
	}
	previous := policy.Revision
	policy.ReadEnabled = false
	if _, err = f.store.SavePolicy(f.ctx, f.scope, policy, previous); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadSource(f.ctx, f.scope, version.Sources[0]); err == nil {
		t.Fatal("read-disabled source remained available")
	}
}
