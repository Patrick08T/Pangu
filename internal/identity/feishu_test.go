package identity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFeishuHTTPFailureReportsStatusWithoutResponseSecrets(t *testing.T) {
	client := feishuClient()
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 400, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"code":20049,"error_description":"private-secret"}`)), Request: r}, nil
	})
	_, err := exchange(context.Background(), client, Config{PublicURL: "https://pangu.example"}, "private-code")
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "code 20049") || strings.Contains(err.Error(), "private-") {
		t.Fatalf("身份接口诊断应保留 HTTP 状态且不泄漏响应和凭据: %v", err)
	}
}

func TestFeishuExchangeUsesV3AndDoesNotExposeTokens(t *testing.T) {
	cfg := Config{AppID: "cli_test", AppSecret: "test-secret", PublicURL: "https://pangu.example", Tenant: "tenant-test"}
	calls := 0
	client := feishuClient()
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"code":0,"access_token":"private-access-token","expires_in":7200,"token_type":"Bearer"}`
		if calls == 1 {
			if r.URL.String() != "https://accounts.feishu.cn/oauth/v3/token" || r.Method != "POST" {
				t.Errorf("错误端点: %s", r.URL)
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Has("code_verifier") || r.Form.Get("client_secret") != "test-secret" || r.Form.Get("redirect_uri") != "https://pangu.example/api/auth/callback" {
				t.Errorf("授权参数缺失")
			}
		} else {
			if r.URL.String() != "https://open.feishu.cn/open-apis/authen/v1/user_info" || r.Header.Get("Authorization") != "Bearer private-access-token" {
				t.Error("错误身份查询")
			}
			body = `{"code":0,"data":{"open_id":"ou_test","tenant_key":"tenant-test","name":"测试成员"}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	user, err := exchange(context.Background(), client, cfg, "test-code")
	if err != nil || user.OpenID != "ou_test" || user.Name != "测试成员" || calls != 2 {
		t.Fatalf("飞书授权失败: %+v %v", user, err)
	}
}

func TestFeishuRejectsUntrustedTenantAndRedirect(t *testing.T) {
	cfg := Config{AppID: "cli_test", AppSecret: "secret", PublicURL: "https://pangu.example", Tenant: "allowed"}
	client := feishuClient()
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://evil.example/"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	if _, err := exchange(context.Background(), client, cfg, "code"); err == nil {
		t.Fatal("飞书请求跟随了不可信重定向")
	}
	cfg.PublicURL = "http://public.example"
	if err := cfg.Validate(); err == nil {
		t.Fatal("允许远程明文登录")
	}
	cfg.PublicURL = "http://127.0.0.1:8080"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestFeishuVerifiedIdentityMustBelongToAllowedTenant(t *testing.T) {
	client := feishuClient()
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"code":0,"access_token":"private","expires_in":7200,"token_type":"Bearer"}`
		if r.Method == "GET" {
			body = `{"code":0,"data":{"open_id":"ou_other","tenant_key":"other-tenant","name":"外部用户"}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	_, err := exchange(context.Background(), client, Config{AppID: "app", AppSecret: "secret", PublicURL: "https://pangu.example", Tenant: "allowed"}, "code")
	if !errors.Is(err, ErrForbidden) {
		t.Fatal("真实用户信息响应的陌生租户被允许", err)
	}
}
