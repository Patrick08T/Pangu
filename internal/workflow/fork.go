package workflow

type Fork struct {
	ReportHashes   map[string]string     `json:"report_hashes,omitempty"`
	ReportNodes    map[string]bool       `json:"report_nodes,omitempty"`
	RequestHash    string                `json:"request_hash"`
	OriginCommit   string                `json:"origin_commit"`
	Checkpoints    map[string]string     `json:"checkpoints,omitempty"`
	CheckpointNode string                `json:"checkpoint_node"`
	Commit         string                `json:"commit"`
	Reused         map[string]NodeOutput `json:"reused,omitempty"`
}

type ForkInput struct {
	CheckpointNode string     `json:"checkpoint_node"`
	Workflow       Definition `json:"workflow"`
}
