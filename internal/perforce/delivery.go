package perforce

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (s Snapshot) PendingChange(ctx context.Context) (int64, error) {
	marker := s.marker() + " review"
	if s.LocalRoot != "" {
		marker = "Pangu local run " + s.RunID
	}
	rows, err := Command(ctx, s.Config, s.Client, "", "changes", "-s", "pending", "-c", s.Client, "-l")
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		if strings.TrimSpace(row["desc"]) == marker {
			return strconv.ParseInt(row["change"], 10, 64)
		}
	}
	form := fmt.Sprintf("Change: new\nClient: %s\nUser: %s\nStatus: pending\nDescription:\n\t%s\n", s.Client, s.Config.User, marker)
	if _, err = Command(ctx, s.Config, s.Client, form, "change", "-i"); err != nil {
		return 0, err
	}
	rows, err = Command(ctx, s.Config, s.Client, "", "changes", "-s", "pending", "-c", s.Client, "-l")
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		if strings.TrimSpace(row["desc"]) == marker {
			return strconv.ParseInt(row["change"], 10, 64)
		}
	}
	return 0, errors.New("待审核 changelist 创建结果不明确，可重试查找同一编号")
}

func (s Snapshot) File(name string) string {
	prefix := "//" + s.Client + "/"
	if s.Config.Scope != "." {
		prefix += s.Config.Scope + "/"
	}
	return prefix + strings.NewReplacer("%", "%25", "@", "%40", "#", "%23", "*", "%2A").Replace(name)
}

func (s Snapshot) Reconcile(ctx context.Context, change int64, paths []string) error {
	for _, name := range paths {
		if _, err := Command(ctx, s.Config, s.Client, "", "reconcile", "-I", "-c", strconv.FormatInt(change, 10), s.File(name)); err != nil {
			return err
		}
	}
	rows, err := Command(ctx, s.Config, s.Client, "", "opened", "-c", strconv.FormatInt(change, 10))
	if err != nil {
		return err
	}
	expected := map[string]bool{}
	for _, name := range paths {
		expected[s.File(name)] = true
	}
	if len(rows) != len(paths) {
		return errors.New("changelist 文件数量与运行差异不一致，拒绝 shelve")
	}
	for _, row := range rows {
		if !expected[row["clientFile"]] {
			return errors.New("changelist 混入额外文件，拒绝 shelve")
		}
	}
	_, err = Command(ctx, s.Config, s.Client, "", "shelve", "-Af", "-c", strconv.FormatInt(change, 10))
	return err
}
