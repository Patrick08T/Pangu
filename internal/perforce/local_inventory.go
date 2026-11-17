package perforce

import (
	"context"
	"time"
)

// LocalInventory is complete only when the operation returns no error.
type LocalInventory struct {
	Files            []string
	PathBytes        int64
	ExcludedMetadata int
	Stages           []LocalInventoryStage
}

type LocalInventoryStage struct {
	Name     string
	Records  int
	Duration time.Duration
}

type localInventoryBudgets struct{ info, fstat, reconcile, total time.Duration }

func defaultInventoryBudgets() localInventoryBudgets {
	return localInventoryBudgets{45 * time.Second, 2 * time.Minute, 30 * time.Minute, 35 * time.Minute}
}

func defaultLocalFileLimits() localFileLimits {
	return localFileLimits{record: 1 << 20, command: 512 << 20, files: 2_000_000, paths: 256 << 20}
}

// LocalInventory freezes one command session; it never caches across snapshots.
func (s Snapshot) LocalInventory(ctx context.Context) (LocalInventory, error) {
	return s.localInventory(ctx, defaultLocalFileLimits(), defaultInventoryBudgets())
}

func (s Snapshot) localInventory(ctx context.Context, limits localFileLimits, budgets localInventoryBudgets) (LocalInventory, error) {
	ctx, cancel := context.WithTimeout(ctx, budgets.total)
	defer cancel()
	session, err := s.newLocalSession()
	if err != nil {
		return LocalInventory{}, err
	}
	if err = s.checkInventoryRoot(ctx); err != nil {
		return LocalInventory{}, err
	}
	info, err := session.loadInfo(ctx, budgets.info)
	if err != nil {
		return LocalInventory{}, err
	}
	result := LocalInventory{Stages: []LocalInventoryStage{info}}
	files := localFileSet{snapshot: s, limits: limits, paths: map[string]struct{}{}, ctx: ctx}
	for _, command := range []struct {
		name   string
		budget time.Duration
		args   []string
	}{
		{"fstat -Rh", budgets.fstat, []string{"fstat", "-Rh", "-T", "clientFile", s.Config.Files(s.Client)}},
		{"fstat -Ro", budgets.fstat, []string{"fstat", "-Ro", "-T", "clientFile", s.Config.Files(s.Client)}},
		{"reconcile", budgets.reconcile, []string{"reconcile", "-n", "-a", "-l", s.Config.Files(s.Client)}},
	} {
		stage, err := session.stage(ctx, command.budget, command.name, limits, files.add, command.args...)
		if err != nil {
			return LocalInventory{}, err
		}
		result.Stages = append(result.Stages, stage)
	}
	result.Files, err = localSortedPaths(ctx, files.paths)
	if err != nil {
		return LocalInventory{}, err
	}
	result.PathBytes, result.ExcludedMetadata = int64(files.bytes), files.excluded
	return result, nil
}

func (s *localCommandSession) loadInfo(ctx context.Context, budget time.Duration) (LocalInventoryStage, error) {
	limits := localFileLimits{record: 1 << 20, command: 8 << 20}
	return s.stage(ctx, budget, "info", limits, func(row Record) error { s.info = append(s.info, row); return nil }, "info")
}

func (s *localCommandSession) stage(ctx context.Context, budget time.Duration, name string, limits localFileLimits, consume func(Record) error, args ...string) (LocalInventoryStage, error) {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	started := time.Now()
	stage := LocalInventoryStage{Name: name}
	err := s.stream(ctx, limits, func(row Record) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		stage.Records++
		return consume(row)
	}, args...)
	stage.Duration = time.Since(started)
	return stage, err
}
