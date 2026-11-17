package perforce

import (
	"context"
	"errors"
	"slices"
	"strconv"
)

// TrackLocal 仅打开本次差异中的新文件；已打开文件保持原 changelist。
func (s Snapshot) TrackLocal(ctx context.Context, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return []string{}, nil
	}
	return s.trackLocal(ctx, paths)
}

func (s Snapshot) ShelveLocal(ctx context.Context, allowed []string) (bool, error) {
	if s.WorkChange == 0 {
		return false, nil
	}
	rows, err := s.localCommand(ctx, "opened", "-c", strconv.FormatInt(s.WorkChange, 10))
	if err != nil || len(rows) == 0 {
		return false, err
	}
	for _, row := range rows {
		name, err := s.localName(row["clientFile"])
		if err != nil {
			return false, err
		}
		if !slices.Contains(allowed, name) {
			return false, errors.New("本轮 changelist 混入其他文件，请先在 P4V 审查清单归属")
		}
	}
	_, err = s.localCommand(ctx, "shelve", "-Af", "-c", strconv.FormatInt(s.WorkChange, 10))
	return err == nil, err
}
