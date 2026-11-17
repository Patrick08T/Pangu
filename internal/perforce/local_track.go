package perforce

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"strings"
)

type localTrackSet struct {
	values map[string]string
	bytes  int
	limits localFileLimits
}

func localTrackingKey(name string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(name)
	}
	return name
}

func (set *localTrackSet) put(name, value string) error {
	key := localTrackingKey(name)
	if old, ok := set.values[key]; ok {
		if old != value {
			return errors.New("P4 文件归属或大小写存在冲突")
		}
		return nil
	}
	if len(set.values) >= set.limits.files || len(key) > set.limits.paths-set.bytes {
		return errors.New("P4 跟踪清单超过唯一文件或路径预算")
	}
	set.values[strings.Clone(key)] = strings.Clone(value)
	set.bytes += len(key)
	return nil
}

func (s Snapshot) trackPaths(ctx context.Context, paths []string, limits localFileLimits) ([]string, error) {
	set := localTrackSet{values: map[string]string{}, limits: limits}
	unique := map[string]struct{}{}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(path) > limits.record {
			return nil, errors.New("P4 单条跟踪路径超过 1MiB")
		}
		file := s.File(path)
		if len(file)+1 > limits.record {
			return nil, errors.New("P4 单条跟踪路径超过 1MiB")
		}
		name, err := s.localNameContext(ctx, file)
		if err != nil {
			return nil, err
		}
		if name != path {
			return nil, errors.New("P4 跟踪文件名不是规范项目相对路径")
		}
		if err = set.put(name, name); err != nil {
			return nil, err
		}
		unique[name] = struct{}{}
	}
	return localSortedPaths(ctx, unique)
}

func (s Snapshot) openedChanges(ctx context.Context, session *localCommandSession, limits localFileLimits) (map[string]string, error) {
	set := localTrackSet{values: map[string]string{}, limits: limits}
	_, err := session.stage(ctx, defaultInventoryBudgets().fstat, "opened", limits, func(row Record) error {
		if row["clientFile"] == "" {
			return nil
		}
		name, err := s.localNameContext(ctx, row["clientFile"])
		if err != nil {
			return err
		}
		change := row["change"]
		if n, err := strconv.ParseInt(change, 10, 64); change != "default" && (err != nil || n <= 0 || strconv.FormatInt(n, 10) != change) {
			return errors.New("P4 已打开文件缺少有效 changelist 归属")
		}
		return set.put(name, change)
	}, "opened", s.Config.Files(s.Client))
	return set.values, err
}

func (s Snapshot) trackGroups(ctx context.Context, paths []string, opened map[string]string) (map[string][]string, []string, error) {
	groups, others := map[string][]string{}, map[string]struct{}{}
	work := strconv.FormatInt(s.WorkChange, 10)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		change := opened[localTrackingKey(path)]
		if change == "" {
			if s.WorkChange <= 0 {
				return nil, nil, errors.New("本轮 changelist 尚未创建")
			}
			change = work
		}
		groups[change] = append(groups[change], path)
		if change != work {
			others[change] = struct{}{}
		}
	}
	other, err := localSortedPaths(ctx, others)
	return groups, other, err
}

func (s Snapshot) reconcileBatches(ctx context.Context, session *localCommandSession, change string, paths []string) error {
	for start := 0; start < len(paths); {
		batch, bytes := []string{}, 0
		for start < len(paths) && len(batch) < 128 {
			if err := ctx.Err(); err != nil {
				return err
			}
			file := s.File(paths[start])
			if len(file)+1 > 1<<20 {
				return errors.New("P4 单条跟踪路径超过 1MiB")
			}
			if len(batch) > 0 && bytes+len(file)+1 > 16<<10 {
				break
			}
			batch = append(batch, file)
			bytes += len(file) + 1
			start++
		}
		child, cancel := context.WithTimeout(ctx, defaultInventoryBudgets().reconcile)
		err := session.streamFiles(child, defaultLocalFileLimits(), []string{"reconcile", "-c", change}, batch)
		cancel()
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (s Snapshot) trackLocal(ctx context.Context, paths []string) ([]string, error) {
	limits := defaultLocalFileLimits()
	paths, err := s.trackPaths(ctx, paths, limits)
	if err != nil {
		return nil, err
	}
	session, err := s.newLocalSession()
	if err != nil {
		return nil, err
	}
	if _, err = session.loadInfo(ctx, defaultInventoryBudgets().info); err != nil {
		return nil, err
	}
	opened, err := s.openedChanges(ctx, session, limits)
	if err != nil {
		return nil, err
	}
	groups, other, err := s.trackGroups(ctx, paths, opened)
	if err != nil {
		return nil, err
	}
	keys := map[string]struct{}{}
	for change := range groups {
		keys[change] = struct{}{}
	}
	changes, err := localSortedPaths(ctx, keys)
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		if err := s.reconcileBatches(ctx, session, change, groups[change]); err != nil {
			return nil, err
		}
	}
	return other, nil
}
