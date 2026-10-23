package scheduler_test

import (
	"context"
	"net/http"
)

// 调度实验单独验证执行语义；身份和越权在 identity/api 的数据库测试中验证。
type integrationAccess struct{}

func (integrationAccess) Handler(next http.Handler) http.Handler                  { return next }
func (integrationAccess) Authorize(context.Context, string, string, string) error { return nil }
