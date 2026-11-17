package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type stopOnlyEvidence struct {
	proof StopProof
	err   error
}

func (e stopOnlyEvidence) ReadStopTx(context.Context, pgx.Tx, Scope, string) (StopProof, error) {
	return e.proof, e.err
}
func (e stopOnlyEvidence) ReadExecutionTx(context.Context, pgx.Tx, Scope, string) (MachineResult, error) {
	return MachineResult{}, errors.New("scoring body is gone")
}
func (e stopOnlyEvidence) ReadObservationTx(context.Context, pgx.Tx, Scope, string) (BusinessEvidence, error) {
	return BusinessEvidence{}, ErrChecker
}

func TestStopReaderIsIndependentOfScoringBodyAndPreservesErrors(t *testing.T) {
	proof := StopProof{RequestID: uuid.NewString(), RunID: uuid.NewString(), StopConfirmed: true}
	service := Service{Evidence: stopOnlyEvidence{proof: proof}}
	actual, err := service.readStopTx(context.Background(), nil, Scope{}, proof.RequestID)
	if err != nil || actual != proof {
		t.Fatal("stop reader depended on score reader", actual, err)
	}
	failure := errors.New("storage unavailable")
	service.Evidence = stopOnlyEvidence{err: failure}
	if _, err = service.readStopTx(context.Background(), nil, Scope{}, proof.RequestID); !errors.Is(err, failure) {
		t.Fatal("storage error replaced", err)
	}
}

func TestStopOnlyLedgerValidatesIdentityWithoutFabricatingScore(t *testing.T) {
	intent := &intentRecord{Intent: Intent{Generation: 3, ExperimentHash: hashJSON("experiment")}}
	execution := cancellationExecution{request: uuid.NewString(), input: hashJSON("input")}
	proof := StopProof{RequestID: execution.request, RunID: uuid.NewString(), Status: "completed", Generation: 3, ExperimentHash: intent.ExperimentHash, ExecutionInputHash: execution.input, SourceHash: hashJSON("runtime stopped"), StopConfirmed: true}
	execution.result = jsonBytes(map[string]any{"stop_proof": proof})
	if ok, err := persistedRuntimeStop(intent, execution); err != nil || !ok {
		t.Fatal("valid runtime-only stop rejected", ok, err)
	}
	for _, name := range []string{"request", "run", "generation", "experiment", "input", "source", "status", "stopped"} {
		t.Run(name, func(t *testing.T) {
			bad := proof
			switch name {
			case "request":
				bad.RequestID = uuid.NewString()
			case "run":
				bad.RunID = "not-uuid"
			case "generation":
				bad.Generation++
			case "experiment":
				bad.ExperimentHash = "different"
			case "input":
				bad.ExecutionInputHash = "different"
			case "source":
				bad.SourceHash = "not-a-hash"
			case "status":
				bad.Status = "running"
			case "stopped":
				bad.StopConfirmed = false
			}
			if validRuntimeStop(intent, execution, bad) {
				t.Fatal("unproven stop accepted")
			}
		})
	}
}
