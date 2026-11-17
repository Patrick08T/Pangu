package experience

type ArtifactReference struct {
	ID   string `json:"id"`
	Hash string `json:"hash"`
}

type Artifact struct {
	ID                 string              `json:"id"`
	ProjectID          string              `json:"project_id"`
	RunID              string              `json:"run_id"`
	NodeID             string              `json:"node_id"`
	Kind               string              `json:"kind"`
	Hash               string              `json:"hash"`
	InputHash          string              `json:"input_hash"`
	ExecutionInputHash string              `json:"execution_input_hash,omitempty"`
	Generation         int64               `json:"generation"`
	ProofVersion       int                 `json:"proof_version"`
	References         []Reference         `json:"references"`
	Parents            []ArtifactReference `json:"parents"`
	InvalidReason      string              `json:"invalid_reason"`
}

func ValidateArtifact(root ArtifactReference, lookup func(string) (Artifact, error), check func([]Reference) error) error {
	if lookup == nil || check == nil {
		return ErrForbidden
	}
	return validateArtifact(root, lookup, check, map[string]bool{}, 0)
}

func validateArtifact(ref ArtifactReference, lookup func(string) (Artifact, error), check func([]Reference) error, path map[string]bool, depth int) error {
	if ref.ID == "" || ref.Hash == "" || path[ref.ID] || depth > 1024 {
		return ErrInvalidated
	}
	artifact, err := lookup(ref.ID)
	if err != nil {
		return err
	}
	if artifact.ID != ref.ID || artifact.Hash != ref.Hash || artifact.InputHash == "" ||
		artifact.ProofVersion != 1 || artifact.InvalidReason != "" {
		return ErrInvalidated
	}
	if err := check(artifact.References); err != nil {
		return err
	}
	path[ref.ID] = true
	defer delete(path, ref.ID)
	for _, parent := range artifact.Parents {
		if err := validateArtifact(parent, lookup, check, path, depth+1); err != nil {
			return err
		}
	}
	return nil
}
