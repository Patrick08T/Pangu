package perforce

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Template struct {
	Stream  string   `json:"stream,omitempty"`
	LineEnd string   `json:"line_end"`
	View    []string `json:"view"`
}

type Snapshot struct {
	Config      Config   `json:"config"`
	Template    Template `json:"template"`
	Baseline    int64    `json:"baseline"`
	Client      string   `json:"client"`
	Commit      string   `json:"commit"`
	ForDelivery bool     `json:"-"`
	LocalRoot   string   `json:"local_path,omitempty"`
	RunID       string   `json:"run_id,omitempty"`
	WorkChange  int64    `json:"work_change,omitempty"`
	LocalCommit string   `json:"local_commit,omitempty"`
}

var viewLine = regexp.MustCompile(`^("[^"\r\n]+"|\S+)\s+("[^"\r\n]+"|\S+)$`)

func Capture(ctx context.Context, c Config) (*Snapshot, error) {
	records, err := Command(ctx, c, c.Client, "", "client", "-o", c.Client)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 || records[0]["Update"] == "" {
		return nil, errors.New("模板 Client 不存在")
	}
	r := records[0]
	t := Template{Stream: r["Stream"], LineEnd: r["LineEnd"], View: []string{}}
	for i := 0; r[fmt.Sprint("View", i)] != ""; i++ {
		t.View = append(t.View, r[fmt.Sprint("View", i)])
	}
	if _, err = remap(t.View, c.Client, "pangu-check"); err != nil {
		return nil, err
	}
	changes, err := Command(ctx, c, c.Client, "", "changes", "-s", "submitted", "-m", "1", c.Files(c.Client))
	if err != nil {
		return nil, err
	}
	var baseline int64
	if len(changes) > 0 {
		baseline, _ = strconv.ParseInt(changes[0]["change"], 10, 64)
	}
	if baseline <= 0 {
		return nil, errors.New("项目范围内没有可读取的已提交基线")
	}
	files, err := Command(ctx, c, c.Client, "", "files", "-e", "-m", "1", c.Files(c.Client)+"@"+strconv.FormatInt(baseline, 10))
	if err != nil {
		return nil, err
	}
	if len(files) != 1 || files[0]["depotFile"] == "" {
		return nil, errors.New("该已提交基线中没有可同步的项目文件")
	}
	return &Snapshot{Config: c, Template: t, Baseline: baseline}, nil
}

func remap(view []string, old, next string) ([]string, error) {
	if len(view) == 0 {
		return nil, errors.New("模板 Client 没有有效映射")
	}
	out := []string{}
	for _, line := range view {
		parts := viewLine.FindStringSubmatch(line)
		if len(parts) != 3 {
			return nil, errors.New("无法安全解析 Client 映射")
		}
		rhs := strings.Trim(parts[2], `"`)
		prefix := "//" + old + "/"
		if !strings.HasPrefix(rhs, prefix) || strings.Contains(rhs, "/../") {
			return nil, errors.New("Client 映射超出模板范围")
		}
		out = append(out, parts[1]+` "//`+next+"/"+strings.TrimPrefix(rhs, prefix)+`"`)
	}
	return out, nil
}

func (s Snapshot) Prepare(ctx context.Context, root string) (string, error) {
	if !filepath.IsAbs(root) || strings.ContainsAny(root, "\r\n") || s.Baseline <= 0 || !strings.HasPrefix(s.Client, "pangu-") {
		return "", errors.New("独立 Client 参数无效")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || !strings.EqualFold(filepath.Clean(root), filepath.Clean(resolved)) {
		return "", errors.New("独立 Client 目录被替换为链接，拒绝同步")
	}
	if err := s.ensureClient(ctx, root); err != nil {
		return "", err
	}
	args := []string{"sync", "-q"}
	if !s.ForDelivery {
		args = append(args, "-f")
	}
	args = append(args, s.Config.Files(s.Client)+"@"+strconv.FormatInt(s.Baseline, 10))
	_, err = Command(ctx, s.Config, s.Client, "", args...)
	if err == nil && !s.ForDelivery {
		err = s.cleanBaseline(ctx)
	}
	return filepath.Join(root, filepath.FromSlash(s.Config.Scope)), err
}

func (s Snapshot) cleanBaseline(ctx context.Context) error {
	for _, args := range [][]string{{"opened", s.Config.Files(s.Client)}, {"reconcile", "-n", "-I", s.Config.Files(s.Client)}} {
		rows, err := Command(ctx, s.Config, s.Client, "", args...)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row["action"] != "" {
				return errors.New("P4 基线工作区含额外或已打开的文件，拒绝导入")
			}
		}
	}
	return nil
}

func (s Snapshot) ensureClient(ctx context.Context, root string) error {
	records, err := Command(ctx, s.Config, s.Client, "", "client", "-o", s.Client)
	if err != nil {
		return err
	}
	if len(records) != 1 {
		return errors.New("无法读取独立 Client")
	}
	if records[0]["Update"] != "" {
		return s.checkOwned(records[0], root)
	}
	view, err := remap(s.Template.View, s.Config.Client, s.Client)
	if err != nil {
		return err
	}
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	lineEnd := s.Template.LineEnd
	if lineEnd != "local" && lineEnd != "unix" && lineEnd != "win" && lineEnd != "mac" && lineEnd != "share" {
		return errors.New("模板换行格式无效")
	}
	form := fmt.Sprintf("Client: %s\nOwner: %s\nHost: %s\nDescription:\n\t%s\nRoot: %s\nOptions: allwrite noclobber nocompress unlocked nomodtime normdir\nLineEnd: %s\n", s.Client, s.Config.User, host, s.marker(), root, lineEnd)
	if s.Template.Stream != "" {
		if !strings.HasPrefix(s.Template.Stream, "//") || strings.ContainsAny(s.Template.Stream, "\r\n\t") {
			return errors.New("Stream 标识无效")
		}
		form += fmt.Sprintf("Stream: %s\n", s.Template.Stream)
		if !s.ForDelivery {
			form += fmt.Sprintf("StreamAtChange: %d\n", s.Baseline)
		}
	} else {
		form += "View:\n\t" + strings.Join(view, "\n\t") + "\n"
	}
	_, err = Command(ctx, s.Config, s.Client, form, "client", "-i")
	if err != nil {
		return err
	}
	records, err = Command(ctx, s.Config, s.Client, "", "client", "-o", s.Client)
	if err != nil {
		return err
	}
	if len(records) != 1 {
		return errors.New("创建后无法核验独立 Client")
	}
	return s.checkOwned(records[0], root)
}

func (s Snapshot) marker() string {
	return "Pangu managed workspace " + s.Client + " baseline " + strconv.FormatInt(s.Baseline, 10)
}

func (s Snapshot) checkOwned(r Record, root string) error {
	host, _ := os.Hostname()
	if !strings.EqualFold(filepath.Clean(r["Root"]), filepath.Clean(root)) || r["Owner"] != s.Config.User || r["Host"] != host || strings.TrimSpace(r["Description"]) != s.marker() {
		return errors.New("独立 Client 的所有者、主机、目录或归属标记不匹配，拒绝接管")
	}
	if r["Stream"] != s.Template.Stream {
		return errors.New("独立 Client 的 Stream 已变更")
	}
	if s.Template.Stream != "" {
		expected := ""
		if !s.ForDelivery {
			expected = strconv.FormatInt(s.Baseline, 10)
		}
		if r["StreamAtChange"] != expected {
			return errors.New("Stream 基线已变更")
		}
	}
	view, err := remap(s.Template.View, s.Config.Client, s.Client)
	if err != nil {
		return err
	}
	for i, line := range view {
		want, got := viewLine.FindStringSubmatch(line), viewLine.FindStringSubmatch(r[fmt.Sprint("View", i)])
		if len(got) != 3 || strings.Trim(want[1], `"`) != strings.Trim(got[1], `"`) || strings.Trim(want[2], `"`) != strings.Trim(got[2], `"`) {
			return errors.New("独立 Client 映射已变更")
		}
	}
	if r[fmt.Sprint("View", len(view))] != "" {
		return errors.New("独立 Client 存在额外映射")
	}
	return nil
}

func (s Snapshot) RemoveClient(ctx context.Context, root string) error {
	records, err := Command(ctx, s.Config, s.Client, "", "client", "-o", s.Client)
	if err != nil {
		return err
	}
	if len(records) != 1 {
		return errors.New("独立 Client 查询结果无效")
	}
	if records[0]["Update"] == "" {
		return nil
	}
	if err = s.checkOwned(records[0], root); err != nil {
		return err
	}
	_, err = Command(ctx, s.Config, s.Client, "", "client", "-d", s.Client)
	return err
}
