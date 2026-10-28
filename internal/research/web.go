package research

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	ranges := []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20"}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, prefix := range ranges {
		if netip.MustParsePrefix(prefix).Contains(ip) {
			return false
		}
	}
	return true
}

func webURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" || (u.Port() != "" && u.Port() != "443") {
		return nil, errors.New("只允许无凭据、无片段的 HTTPS 公共网页（443 端口）")
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !publicIP(ip) {
		return nil, errors.New("拒绝内网或特殊地址")
	}
	return u, nil
}

func dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" {
		return nil, errors.New("拒绝非 HTTPS 目标")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("公共网页 DNS 解析失败")
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, errors.New("DNS 包含内网或特殊地址")
		}
	}
	// 直接连接经过校验的地址，避免校验后再次解析产生 DNS 重绑定。
	dialer := net.Dialer{Timeout: 10 * time.Second}
	var failures []error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		failures = append(failures, err)
	}
	return nil, errors.Join(failures...)
}

func webClient() *http.Client {
	return &http.Client{Transport: &http.Transport{DialContext: dialPublic, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second, MaxResponseHeaderBytes: 64 * 1024, DisableKeepAlives: true},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 3 {
				return errors.New("网页重定向超过 3 次")
			}
			_, err := webURL(req.URL.String())
			return err
		}}
}

func fetch(ctx context.Context, client *http.Client, address string) (string, string, error) {
	u, err := webURL(address)
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Pangu-Research/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", errors.New("网页获取失败（网络、证书或目标被拒绝）")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("网页返回 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil || len(data) > 2*1024*1024 {
		return "", "", errors.New("网页读取失败或超过 2 MiB")
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	text := string(data)
	if strings.HasPrefix(contentType, "text/html") {
		text = htmlText(text)
	} else if !strings.HasPrefix(contentType, "text/plain") {
		return "", "", errors.New("仅支持 HTML 或纯文本网页")
	}
	if len(text) > 64*1024 {
		return "", "", errors.New("网页正文超过 64 KiB；请提供更聚焦的来源")
	}
	return text, resp.Request.URL.String(), nil
}

func htmlText(raw string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(raw))
	var out strings.Builder
	hidden := 0
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			break
		}
		token := tokenizer.Token()
		if kind == html.StartTagToken && (token.Data == "script" || token.Data == "style" || token.Data == "noscript") {
			hidden++
		}
		if kind == html.EndTagToken && (token.Data == "script" || token.Data == "style" || token.Data == "noscript") && hidden > 0 {
			hidden--
			continue
		}
		if kind == html.TextToken && hidden == 0 {
			out.WriteString(token.Data)
			out.WriteByte('\n')
		}
	}
	return strings.Join(strings.Fields(out.String()), " ")
}
