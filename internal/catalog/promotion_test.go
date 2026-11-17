package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestAutomaticPublishRejectsDraftAndBaselineDrift(t *testing.T) {
	spec := Spec{Agent: &workflow.Node{Instruction: "public"}}
	item := Object{ID: "object", Revision: 4, NewObject: NewObject{Spec: spec}}
	public := Version{ID: "public", ObjectID: item.ID, SourceRevision: 4, Spec: spec}
	target := PublishTarget{ObjectID: item.ID, ExpectedRevision: 4, PublicBaseline: "public"}
	if err := validatePublishTarget(item, public, public, public, target); err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*Object, *Version, *Version, *PublishTarget){
		"draft revision": func(o *Object, p, d *Version, r *PublishTarget) { o.Revision++ },
		"draft content": func(o *Object, p, d *Version, r *PublishTarget) {
			o.Spec = Spec{Agent: &workflow.Node{Instruction: "user draft"}}
		},
		"public drift":    func(o *Object, p, d *Version, r *PublishTarget) { p.ID = "new public" },
		"default drift":   func(o *Object, p, d *Version, r *PublishTarget) { d.ID = "new stable" },
		"foreign object":  func(o *Object, p, d *Version, r *PublishTarget) { r.ObjectID = "other" },
		"previous forged": func(o *Object, p, d *Version, r *PublishTarget) { r.ProjectPrevious = "forged" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			o, p, d, r := item, public, public, target
			mutate(&o, &p, &d, &r)
			if err := validatePublishTarget(o, public, p, d, r); !errors.Is(err, ErrConflict) {
				t.Fatalf("drift accepted: %v", err)
			}
		})
	}
}

func TestAutomaticPublishRequiresTransactionalGuard(t *testing.T) {
	_, err := PublishAutomatic(context.Background(), "namespace", AutomaticPublishRequest{}, nil)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil guard must fail before database: %v", err)
	}
}
