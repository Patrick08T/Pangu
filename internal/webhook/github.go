package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"strings"
	"unicode/utf8"
)

var ErrInvalid = errors.New("事件入口参数无效")
var ErrNotFound = errors.New("事件入口不存在")
var ErrConflict = errors.New("事件标识已对应其他内容")
var ErrDisabled = errors.New("事件入口已停用或创建者权限已撤销")

func Signature(body []byte, signature, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") || len(secret) < 32 {
		return false
	}
	expected, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(expected, mac.Sum(nil))
}

func Input(body []byte, config Config) (string, error) {
	if !utf8.Valid(body) || len(body) > 256*1024 {
		return "", ErrInvalid
	}
	var event struct {
		Action     string `json:"action"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Label struct {
			Name string `json:"name"`
		} `json:"label"`
		Issue struct {
			Number int64  `json:"number"`
			Title  string `json:"title"`
			Body   string `json:"body"`
			URL    string `json:"html_url"`
		} `json:"issue"`
	}
	if json.Unmarshal(body, &event) != nil || event.Action != "labeled" || event.Repository.FullName != config.Repository || event.Label.Name != config.Label {
		return "", ErrInvalid
	}
	issue := event.Issue
	if issue.Number < 1 || issue.URL != fmt.Sprintf("https://github.com/%s/issues/%d", config.Repository, issue.Number) || strings.TrimSpace(issue.Title) == "" || len(issue.Title) > 1024 || len(issue.Body) > 12000 {
		return "", ErrInvalid
	}
	payload, err := json.Marshal(struct {
		Repository string `json:"repository"`
		Number     int64  `json:"issue_number"`
		Title      string `json:"issue_title"`
		Body       string `json:"issue_body"`
		URL        string `json:"issue_url"`
	}{config.Repository, issue.Number, issue.Title, issue.Body, issue.URL})
	if err != nil || diagnostic.Sensitive(payload) {
		return "", ErrInvalid
	}
	return string(payload), nil
}
