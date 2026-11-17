package agent

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino-ext/components/model/openai"
)

var codexHTTPStatus = regexp.MustCompile(`(?i)^unexpected status (400|401|403|404|408|422|429|500|502|503|504)\b`)

func codexFailure(cause error, status int) error {
	var existing *workflow.Failure
	if errors.As(cause, &existing) || errors.Is(cause, workflow.ErrStopUnconfirmed) {
		return cause
	}
	var exit interface{ ExitCode() int }
	if errors.As(cause, &exit) && exit.ExitCode() == 88 {
		return &workflow.Failure{Kind: "trace", Cause: cause}
	}
	if errors.As(cause, &exit) && exit.ExitCode() == 87 {
		return &workflow.Failure{Kind: "tool_authority", Cause: cause}
	}
	kind, retry := "parameter", false
	switch status {
	case 400, 404, 422:
	case 401, 403:
		kind = "authentication"
	case 402:
		kind = "quota"
	case 429:
		kind, retry = "rate_limit", true
	case 408:
		kind, retry = "model_network", true
	case 500, 502, 503, 504:
		kind, retry = "model_unavailable", true
	default:
		return cause
	}
	return &workflow.Failure{Kind: kind, Retry: retry, Cause: cause}
}

func codexStatus(message string) int {
	var event struct {
		Type, Message string
		Status        int
		Error         struct{ Code string }
	}
	if json.Unmarshal([]byte(message), &event) == nil && event.Type == "error" {
		if event.Error.Code == "insufficient_quota" || event.Error.Code == "billing_hard_limit_reached" {
			return 402
		}
		if event.Error.Code == "model_not_found" {
			return 400
		}
		if event.Status != 0 {
			return event.Status
		}
		message = event.Message
	}
	match := codexHTTPStatus.FindStringSubmatch(message)
	if len(match) != 2 {
		if message == "Selected model is at capacity. Please try a different model." {
			return 503
		}
		if strings.HasPrefix(message, "stream disconnected before completion: error sending request for url (") || strings.HasPrefix(message, "error sending request for url (") || message == "request timed out" {
			return 408
		}
		return 0
	}
	status, _ := strconv.Atoi(match[1])
	return status
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	var existing *workflow.Failure
	if errors.As(err, &existing) {
		return err
	}
	kind, retry := workflow.FailureKind(err), workflow.CanRetry(err)
	var api *openai.APIError
	if errors.As(err, &api) {
		if api.Code == "insufficient_quota" || api.Code == "billing_hard_limit_reached" {
			return &workflow.Failure{Kind: "quota", Cause: err}
		}
		switch api.HTTPStatusCode {
		case 401, 403:
			kind, retry = "authentication", false
		case 400, 404, 422:
			kind, retry = "parameter", false
		case 429:
			kind = "rate_limit"
		case 408:
			kind, retry = "model_network", true
		case 500, 502, 503, 504:
			kind, retry = "model_unavailable", true
		}
	}
	return &workflow.Failure{Kind: kind, Cause: err, Retry: retry}
}
