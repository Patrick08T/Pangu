package scheduler

import (
	"encoding/json"
	"fmt"
)

type privateRunHistory struct {
	RunDetails
	Purpose string `json:"internal_purpose"`
}

func marshalRunHistory(run RunDetails) ([]byte, error) {
	purpose := run.Purpose
	if purpose == "" {
		purpose = "business"
	}
	return json.Marshal(privateRunHistory{RunDetails: run, Purpose: purpose})
}

func unmarshalRunHistory(raw []byte) (*RunDetails, error) {
	var stored privateRunHistory
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	switch stored.Purpose {
	case "", "business", "learning", "evaluation":
	default:
		return nil, fmt.Errorf("unknown persisted run purpose")
	}
	stored.RunDetails.Purpose = stored.Purpose
	return &stored.RunDetails, nil
}
