package workflow

type Check struct {
	Component string `json:"component"`
	Status    string `json:"status"`
}

type Preflight struct {
	Ready  bool    `json:"ready"`
	Checks []Check `json:"checks"`
}
