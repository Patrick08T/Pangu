package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var ErrUnauthenticated = errors.New("请使用飞书登录，或提供有效的 Pangu API Token")
var ErrForbidden = errors.New("没有当前项目或操作的权限")
var ErrInvalid = errors.New("身份管理参数无效")

type Config struct{ AppID, AppSecret, PublicURL, Tenant, AdminOpenID string }
type FeishuUser struct {
	OpenID string `json:"open_id"`
	Tenant string `json:"tenant_key"`
	Name   string `json:"name"`
}

func config() (Config, error) {
	c := Config{AppID: os.Getenv("PANGU_FEISHU_APP_ID"), AppSecret: os.Getenv("PANGU_FEISHU_APP_SECRET"), PublicURL: os.Getenv("PANGU_PUBLIC_URL"), Tenant: os.Getenv("PANGU_FEISHU_TENANT_KEY"), AdminOpenID: os.Getenv("PANGU_FEISHU_ADMIN_OPEN_ID")}
	return c, c.Validate()
}

func (c Config) Validate() error {
	u, err := url.Parse(c.PublicURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Opaque != "" {
		return errors.New("PANGU_PUBLIC_URL 必须是无路径、查询参数或凭据的站点源地址")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return errors.New("远程飞书登录必须使用 HTTPS")
	}
	if c.AppID == "" || c.AppSecret == "" || c.Tenant == "" {
		return errors.New("飞书应用或允许的租户尚未配置")
	}
	return nil
}

func (c Config) callback() string { return c.PublicURL + "/api/auth/callback" }

func feishuClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 32 * 1024, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func exchange(ctx context.Context, client *http.Client, c Config, code string) (FeishuUser, error) {
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {c.AppID}, "client_secret": {c.AppSecret}, "code": {code}, "redirect_uri": {c.callback()}}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://accounts.feishu.cn/oauth/v3/token", strings.NewReader(form.Encode()))
	if err != nil {
		return FeishuUser{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token struct {
		Code        *int   `json:"code"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err = feishuJSON(client, req, &token); err != nil {
		return FeishuUser{}, err
	}
	if token.Code == nil || *token.Code != 0 || token.AccessToken == "" || token.ExpiresIn < 1 || !strings.EqualFold(token.TokenType, "Bearer") {
		return FeishuUser{}, errors.New("飞书授权码无效、过期或应用配置有误")
	}
	req, err = http.NewRequestWithContext(ctx, "GET", "https://open.feishu.cn/open-apis/authen/v1/user_info", nil)
	if err != nil {
		return FeishuUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	var info struct {
		Code *int       `json:"code"`
		Data FeishuUser `json:"data"`
	}
	if err = feishuJSON(client, req, &info); err != nil {
		return FeishuUser{}, err
	}
	if info.Code == nil || *info.Code != 0 || info.Data.OpenID == "" || info.Data.Tenant != c.Tenant || len(info.Data.OpenID) > 128 || len(info.Data.Name) > 256 {
		return FeishuUser{}, ErrForbidden
	}
	return info.Data, nil
}

func feishuJSON(client *http.Client, req *http.Request, out any) error {
	response, err := client.Do(req)
	if err != nil {
		return errors.New("飞书服务请求失败")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		var rejected struct {
			Code int `json:"code"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&rejected)
		return fmt.Errorf("飞书服务拒绝请求（HTTP %d，code %d）", response.StatusCode, rejected.Code)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
	if err != nil || len(body) > 64*1024 {
		return errors.New("飞书响应无法读取或超过上限")
	}
	if err = json.Unmarshal(body, out); err != nil {
		return errors.New("飞书响应格式无效")
	}
	return nil
}
