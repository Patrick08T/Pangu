package workflow

type EvidenceReference struct {
	ManifestPath string `json:"manifest_path"`
	SHA256       string `json:"sha256"`
	RunID        string `json:"run_id"`
	FileCount    int    `json:"file_count"`
}
