package agent

import (
	"errors"
	"regexp"

	"github.com/Patrick08T/Pangu/internal/workflow"
	"github.com/cloudwego/eino-ext/components/model/openai"
)

var codexHTTPStatus = regexp.MustCompile(`(?i)^unexpected status (400|401|403|404|422|429)\b`)

func codexFailure(cause error, message string) error {
	var exit interface{ ExitCode() int }
	if errors.As(cause, &exit) && exit.ExitCode() == 86 {
		return &workflow.Failure{Kind: "tool_timeout", Cause: cause}
	}
	if errors.As(cause, &exit) && exit.ExitCode() == 87 {
		return &workflow.Failure{Kind: "tool_limit", Cause: cause}
	}
	match := codexHTTPStatus.FindStringSubmatch(message)
	if len(match) != 2 {
		return cause
	}
	kind, retry := "parameter", false
	switch match[1] {
	case "401", "403":
		kind = "authentication"
	case "429":
		kind, retry = "rate_limit", true
	}
	return &workflow.Failure{Kind: kind, Retry: retry, Cause: cause}
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
		switch api.HTTPStatusCode {
		case 401, 403:
			kind, retry = "authentication", false
		case 400, 404, 422:
			kind, retry = "parameter", false
		case 429:
			kind = "rate_limit"
		}
	}
	return &workflow.Failure{Kind: kind, Cause: err, Retry: retry}
}
