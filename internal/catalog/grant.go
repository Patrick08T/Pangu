package catalog

import (
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
)

type credentialGrant struct {
	ProjectID string   `json:"project_id"`
	BaseURL   string   `json:"base_url"`
	SecretRef string   `json:"secret_ref"`
	Models    []string `json:"models"`
}

func (c Connection) Granted(project string) error {
	raw := os.Getenv("PANGU_CONNECTION_GRANTS")
	if len(raw) > 64*1024 || raw == "" {
		return invalid("服务端尚未登记项目凭据与端点授权")
	}
	var grants []credentialGrant
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&grants); err != nil {
		return invalid("服务端凭据授权配置无效")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return invalid("服务端凭据授权配置无效")
	}
	for _, g := range grants {
		if g.ProjectID == project && project != "" && g.BaseURL == c.BaseURL && g.SecretRef == c.SecretRef && slices.Contains(g.Models, c.Model) {
			return nil
		}
	}
	return invalid("连接未匹配服务端登记的项目、端点、模型与凭据引用")
}
