package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// EffectiveInputHash identifies logical inputs independently of physical retries.
func EffectiveInputHash(input NodeInput) (string, error) {
	if input.ApprovalGeneration < 0 || input.ApprovalRound < 0 {
		return "", errors.New("审批代际与轮次不能为负")
	}
	input.ApprovalID = ""
	input.RequestID = ""
	if input.ApprovalGeneration == 0 {
		input.ApprovalGeneration = 1
	}
	return fingerprint(input)
}

// OutputFingerprint covers the reusable output, excluding its own proof envelope.
func OutputFingerprint(output NodeOutput) (string, error) {
	output.Lineage = nil
	return fingerprint(output)
}

func fingerprint(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
