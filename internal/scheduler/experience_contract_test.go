package scheduler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/workflow"
)

func TestExperienceSelectionUsesBoundInputAndLogicalIdentity(t *testing.T) {
	input := workflow.NodeInput{ProjectID: "project", Node: workflow.Node{ID: "implement", AgentVersion: "role-v1"}, Input: "request", Parameters: map[string]any{"module": "payments"}, ApprovalGeneration: 2, ApprovalRound: 3, Iteration: &workflow.Iteration{ID: "review", Round: 4}}
	selection, err := experienceSelection("request-id", input, "role-id", "manual")
	if err != nil || selection.RunID != "request-id" || selection.WatermarkGeneration != 1 || selection.Generation != 2 || selection.Round != 4 || selection.ApprovalRevision != 3 || selection.Module != "payments" || selection.RoleID != "role-id" || selection.RoleVersion != "role-v1" || len(selection.BoundInputHash) != 64 {
		t.Fatalf("selection lost frozen logical scope: %+v %v", selection, err)
	}
	input.Experiences = []workflow.Experience{{Reference: workflow.MemoryReference{VersionID: "old-injection"}}}
	retry, err := experienceSelection("request-id", input, "role-id", "manual")
	if err != nil || retry.BoundInputHash != selection.BoundInputHash {
		t.Fatalf("injection recursively changed selection identity: %+v %v", retry, err)
	}
	input.Parameters["module"] = "billing"
	changed, err := experienceSelection("request-id", input, "role-id", "manual")
	if err != nil || changed.BoundInputHash == selection.BoundInputHash {
		t.Fatal("changed bound input reused an earlier selection")
	}
}

func TestExperiencePayloadBindsCanonicalBodyAndProvenance(t *testing.T) {
	content := experience.Content{Kind: "practice", Practice: "Use isolated fixture", Conditions: "database tests"}
	body, _ := json.Marshal(content)
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	ref := experience.Reference{VersionID: "version", BodyHash: hash, DeliveryRevision: 3}
	version := experience.Version{ID: "version", BodyHash: hash, Content: content, Sources: []experience.Source{{ID: "report", Kind: "report", RunID: "source-run", NodeID: "source-node", Hash: "source-hash"}}}
	payload, err := experiencePayload(experience.Usage{References: []experience.Reference{ref}, MatchReason: "scoped_match"}, []experience.Version{version})
	if err != nil || len(payload) != 1 {
		t.Fatalf("lost selected body: %+v %v", payload, err)
	}
	item := payload[0]
	if string(item.Content) != string(body) || item.Reference.BodyHash != hash || item.Reference.DeliveryRevision != 3 || len(item.Sources) != 1 || item.Sources[0].EvidenceHash != "source-hash" || item.Reason != "scoped_match" {
		t.Fatalf("lost frozen payload provenance: %+v", item)
	}
	version.Content.Practice = "tampered"
	if _, err = experiencePayload(experience.Usage{References: []experience.Reference{ref}}, []experience.Version{version}); err == nil {
		t.Fatal("selected reference allowed different body")
	}
}

func TestOutputArtifactRequiresAllParentProofsAndFrozenInput(t *testing.T) {
	parent := workflow.NodeOutput{RunID: "parent-run", Text: "verified"}
	parentHash, _ := workflow.OutputFingerprint(parent)
	parent.Lineage = &workflow.OutputLineage{Version: 1, ArtifactID: "parent-run", InputHash: hashText("parent-input"), OutputHash: parentHash}
	input := workflow.NodeInput{ProjectID: "project", RunID: "root", NodeRunID: "node-run", Node: workflow.Node{ID: "next"}, Parents: map[string]workflow.NodeOutput{"parent": parent}, Experiences: []workflow.Experience{{Reference: workflow.MemoryReference{VersionID: "memory", BodyHash: hashText("body"), DeliveryRevision: 2}}}}
	out := workflow.NodeOutput{RunID: "node-run", Text: "result"}
	artifact, err := outputArtifact(input, out)
	expected, _ := workflow.EffectiveInputHash(input)
	if err != nil || artifact.InputHash != expected || artifact.Generation != 1 || artifact.ProofVersion != 1 || len(artifact.Parents) != 1 || artifact.Parents[0].Hash != parentHash || len(artifact.References) != 1 {
		t.Fatalf("incomplete artifact: %+v %v", artifact, err)
	}
	parent.Text = "changed"
	input.Parents["parent"] = parent
	if _, err = outputArtifact(input, out); err == nil {
		t.Fatal("changed parent reused valid envelope")
	}
	parent.Lineage = nil
	parent.Skipped = true
	input.Parents["parent"] = parent
	if _, err = outputArtifact(input, out); err == nil {
		t.Fatal("skipped parent bypassed provenance requirement")
	}
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
