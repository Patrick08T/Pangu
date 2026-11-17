package perforce

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// CaptureLocal 仅核对本地 Client，不同步、清理或移动用户文件。
func CaptureLocal(ctx context.Context, c Config, root string) (*Snapshot, error) {
	s, err := Capture(ctx, c)
	if err != nil {
		return nil, err
	}
	s.Client, s.LocalRoot = c.Client, root
	if _, err = s.LocalDirectory(ctx); err != nil {
		return nil, err
	}
	rows, err := Command(ctx, c, c.Client, "", "changes", "-s", "submitted", "-m", "1", c.Files(c.Client)+"#have")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("绑定工作区尚无已同步文件，请先在 P4V 同步项目")
	}
	s.Baseline, err = strconv.ParseInt(rows[0]["change"], 10, 64)
	return s, err
}

func (s Snapshot) LocalDirectory(ctx context.Context) (string, error) {
	if !filepath.IsAbs(s.LocalRoot) || s.Client != s.Config.Client {
		return "", errors.New("Perforce 本地目录或 Client 绑定无效")
	}
	rows, err := s.localCommand(ctx, "client", "-o", s.Client)
	if err != nil {
		return "", err
	}
	if len(rows) != 1 || rows[0]["Update"] == "" {
		return "", errors.New("绑定的 Perforce Client 不存在")
	}
	r := rows[0]
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	if r["Host"] != "" && !strings.EqualFold(r["Host"], host) {
		return "", errors.New("Perforce Client 绑定了其他主机")
	}
	matched := false
	for _, key := range []string{"Root", "AltRoots0", "AltRoots1"} {
		rel, err := filepath.Rel(filepath.Clean(r[key]), filepath.Clean(s.LocalRoot))
		matched = matched || r[key] != "" && err == nil && rel == "."
	}
	if !matched {
		return "", errors.New("本地工作区根目录与 Perforce Client 的 Root/AltRoots 不一致")
	}
	var view []string
	for i := 0; r["View"+strconv.Itoa(i)] != ""; i++ {
		view = append(view, r["View"+strconv.Itoa(i)])
	}
	if r["Stream"] != s.Template.Stream || !slices.Equal(view, s.Template.View) {
		return "", errors.New("绑定 Client 的映射发生变化，请重新发起运行")
	}
	dir := filepath.Join(s.LocalRoot, filepath.FromSlash(s.Config.Scope))
	actual, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", errors.New("绑定的本地项目目录不存在或无法访问")
	}
	rel, err := filepath.Rel(dir, actual)
	if err != nil || rel != "." {
		return "", errors.New("绑定项目目录不能通过链接跳转")
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", errors.New("绑定路径不是目录")
	}
	return dir, nil
}

// LocalFiles 只列入已同步、已打开和 P4 未忽略的新文件，保留本地现状。
func (s Snapshot) LocalFiles(ctx context.Context) ([]string, error) {
	result, err := s.LocalInventory(ctx)
	return result.Files, err
}

func (s Snapshot) localName(name string) (string, error) {
	return s.localNameContext(context.Background(), name)
}

func (s Snapshot) localNameContext(ctx context.Context, name string) (string, error) {
	rel, err := s.normalizedLocalNameContext(ctx, name)
	if err != nil {
		return "", err
	}
	for _, part := range strings.Split(rel, "/") {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if strings.EqualFold(part, ".git") {
			return "", errors.New("P4 项目不能包含 Git 元数据")
		}
	}
	return rel, nil
}

func (s Snapshot) localCommand(ctx context.Context, args ...string) ([]Record, error) {
	return commandIn(ctx, s.Config, s.Client, "", s.LocalRoot, args...)
}
