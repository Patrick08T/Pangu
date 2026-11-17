package learning

import (
	"testing"
)

func TestCheckerSupportsSafeScalarContractOnly(t *testing.T) {
	spec := CheckerSpec{Kind: "json_assertion_v1", ContentHash: digest([]byte("content")), SourceKind: "feedback", JSONPointer: "/decision", Expected: []byte(`"accepted"`)}
	if err := validateChecker(spec); err != nil {
		t.Fatal("safe feedback scalar rejected", err)
	}
	if !checkJSON(spec, []byte(`{"decision":"accepted"}`)) || checkJSON(spec, []byte(`{"decision":"rejected"}`)) {
		t.Fatal("checker did not compute actual JSON value")
	}
	for _, expected := range []string{`{"passed":true}`, `[true]`, `null`, `"Bearer sk-test123456789012345678901234567890"`} {
		spec.Expected = []byte(expected)
		if err := validateChecker(spec); err == nil {
			t.Fatal("unsafe or unsupported expected value accepted", expected)
		}
	}
}

func TestCheckerDoesNotRoundDistinctLargeJSONNumbers(t *testing.T) {
	spec := CheckerSpec{JSONPointer: "/value", Expected: []byte(`9007199254740992`)}
	if checkJSON(spec, []byte(`{"value":9007199254740993}`)) {
		t.Fatal("rounded distinct integers to the same claim")
	}
}
