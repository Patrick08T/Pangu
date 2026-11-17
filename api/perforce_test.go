package contract

import (
	"encoding/json"
	"github.com/getkin/kin-openapi/openapi3"
	"testing"
)

func TestProjectSourceContractRejectsMixedAndMissingFields(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData(OpenAPI)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		body  string
		valid bool
	}{
		{`{"vcs":"git","name":"项目","worker_id":"worker","source":"C:/repo","branch":"main"}`, true},
		{`{"vcs":"perforce","name":"项目","worker_id":"worker","perforce":{"port":"localhost:1666","user":"user","client":"daily","scope":"Server"}}`, false},
		{`{"vcs":"git","name":"项目","worker_id":"worker"}`, false},
		{`{"vcs":"perforce","name":"项目","worker_id":"worker","source":"C:/repo","branch":"main"}`, false},
		{`{"vcs":"perforce","name":"项目","worker_id":"worker","source":"C:/repo","perforce":{"port":"localhost:1666","user":"user","client":"daily","scope":"Server"}}`, true},
		{`{"vcs":"perforce","name":"项目","worker_id":"worker","source":"C:/repo","branch":"main","perforce":{"port":"localhost:1666","user":"user","client":"daily","scope":"Server"}}`, false},
	} {
		var value any
		if err = json.Unmarshal([]byte(test.body), &value); err != nil {
			t.Fatal(err)
		}
		if err = doc.Components.Schemas["ProjectRegistration"].Value.VisitJSON(value); (err == nil) != test.valid {
			t.Errorf("来源契约不一致 %s: %v", test.body, err)
		}
	}
}
