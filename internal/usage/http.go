package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"io"
	"net"
	"net/http"
	"syscall"
)

type Transport struct {
	Base        http.RoundTripper
	Kind, Model string
	Price       *workflow.Price
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	id, err := Reserve(req.Context(), t.Kind, t.Model, t.Price)
	if err != nil {
		return nil, err
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	response, err := base.RoundTrip(req)
	if err != nil {
		return nil, transportFailure(t.Kind, err, Finish(id, nil, false))
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
	response.Body.Close()
	if len(raw) > 4*1024*1024 {
		err = errors.New("用量响应超过 4 MiB")
	}
	if err != nil {
		return nil, transportFailure(t.Kind, err, Finish(id, nil, false))
	}
	var tokens *Tokens
	if t.Kind == "model" && response.StatusCode >= 200 && response.StatusCode < 300 {
		tokens = modelTokens(raw)
	}
	if err = Finish(id, tokens, response.StatusCode >= 200 && response.StatusCode < 300); err != nil {
		return nil, err
	}
	response.Body = io.NopCloser(bytes.NewReader(raw))
	return response, nil
}

func modelTokens(raw []byte) *Tokens {
	type details struct {
		Cached int64 `json:"cached_tokens"`
	}
	var payload struct {
		Usage *struct {
			Prompt        *int64  `json:"prompt_tokens"`
			Completion    *int64  `json:"completion_tokens"`
			PromptDetails details `json:"prompt_tokens_details"`
			Input         *int64  `json:"input_tokens"`
			Output        *int64  `json:"output_tokens"`
			InputDetails  details `json:"input_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.Usage == nil {
		return nil
	}
	u := payload.Usage
	if u.Input != nil && u.Output != nil {
		return &Tokens{Input: *u.Input, Output: *u.Output, Cached: u.InputDetails.Cached}
	}
	if u.Prompt != nil && u.Completion != nil {
		return &Tokens{Input: *u.Prompt, Output: *u.Completion, Cached: u.PromptDetails.Cached}
	}
	return nil
}

func transportFailure(kind string, cause, recordErr error) error {
	if recordErr != nil {
		return &workflow.Failure{Kind: "usage", Cause: errors.Join(cause, recordErr)}
	}
	if kind != "model" || errors.Is(cause, context.Canceled) {
		return cause
	}
	var network net.Error
	temporary := errors.As(cause, &network) && (network.Timeout() || network.Temporary())
	for _, transient := range []error{io.EOF, io.ErrUnexpectedEOF, syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.EPIPE} {
		temporary = temporary || errors.Is(cause, transient)
	}
	if temporary {
		return &workflow.Failure{Kind: "model_network", Retry: true, Cause: cause}
	}
	return cause
}
