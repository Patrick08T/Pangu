package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"io"
	"net/http"
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
		return nil, errors.Join(err, Finish(id, nil, false))
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
	response.Body.Close()
	if len(raw) > 4*1024*1024 {
		err = errors.New("用量响应超过 4 MiB")
	}
	if err != nil {
		return nil, errors.Join(err, Finish(id, nil, false))
	}
	var payload struct {
		Usage *struct {
			Input   *int64 `json:"prompt_tokens"`
			Output  *int64 `json:"completion_tokens"`
			Details struct {
				Cached int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	var tokens *Tokens
	if t.Kind == "model" && response.StatusCode >= 200 && response.StatusCode < 300 && json.Unmarshal(raw, &payload) == nil && payload.Usage != nil && payload.Usage.Input != nil && payload.Usage.Output != nil {
		tokens = &Tokens{Input: *payload.Usage.Input, Output: *payload.Usage.Output, Cached: payload.Usage.Details.Cached}
	}
	if err = Finish(id, tokens, response.StatusCode >= 200 && response.StatusCode < 300); err != nil {
		return nil, err
	}
	response.Body = io.NopCloser(bytes.NewReader(raw))
	return response, nil
}
