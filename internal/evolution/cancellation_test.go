package evolution

import (
	"strings"
	"testing"
)

func TestCancellationStopProofRejectsUnboundOrUnknownRun(t *testing.T) {
	intent := &intentRecord{Intent: Intent{Generation: 3, ExperimentHash: strings.Repeat("a", 64)}}
	execution := cancellationExecution{request: "request", input: strings.Repeat("b", 64)}
	result := MachineResult{RequestID: execution.request, Generation: 3, ExperimentHash: intent.ExperimentHash, ExecutionInputHash: execution.input, RunID: "22222222-2222-4222-8222-222222222222", NodeID: "node", SourceHash: strings.Repeat("c", 64), Status: "completed", StopConfirmed: true}
	if !validStopProof(intent, execution, result) {
		t.Fatal("真实绑定terminal拒绝")
	}
	changes := map[string]func(*MachineResult){
		"request":    func(r *MachineResult) { r.RequestID = "other" },
		"generation": func(r *MachineResult) { r.Generation++ },
		"experiment": func(r *MachineResult) { r.ExperimentHash = "other" },
		"input":      func(r *MachineResult) { r.ExecutionInputHash = "other" },
		"run":        func(r *MachineResult) { r.RunID = "unknown" },
		"node":       func(r *MachineResult) { r.NodeID = "" },
		"source":     func(r *MachineResult) { r.SourceHash = strings.Repeat("z", 64) },
		"stop":       func(r *MachineResult) { r.StopConfirmed = false },
		"running":    func(r *MachineResult) { r.Status = "running" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			bad := result
			change(&bad)
			if validStopProof(intent, execution, bad) {
				t.Fatal("未知或不绑定运行释放了claims")
			}
		})
	}
}
