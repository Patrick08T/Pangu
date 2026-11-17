package workspace

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

func localReserveSpace(total uint64) uint64 {
	reserve := total / 20
	if total%20 != 0 {
		reserve++
	}
	return max(uint64(10<<30), reserve)
}

func localRequiredSpace(m localManifest, total uint64) (uint64, error) {
	if m.TotalBytes < 0 || m.PathBytes < 0 || len(m.Files) > localLimits.entries || m.PathBytes > int64(localLimits.paths) {
		return 0, errors.New("本地快照容量参数无效")
	}
	s, p, n := uint64(m.TotalBytes), uint64(m.PathBytes), uint64(len(m.Files))
	parts := []struct{ value, factor uint64 }{{s / 5, 12}, {s%5*12 + 4, 1}, {n, 4096}, {p, 2}}
	need := localReserveSpace(total)
	for i, part := range parts {
		if part.value > math.MaxUint64/part.factor {
			return 0, errors.New("本地快照空间估计溢出")
		}
		value := part.value * part.factor
		if i == 1 {
			value /= 5
		}
		if value > math.MaxUint64-need {
			return 0, errors.New("本地快照空间估计溢出")
		}
		need += value
	}
	return need, nil
}

func checkLocalCaptureCapacity(ctx context.Context, repository string, manifest localManifest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	free, total, err := localVolumeSpace(repository)
	if err != nil {
		return err
	}
	need, err := localRequiredSpace(manifest, total)
	if err != nil {
		return err
	}
	if free < need {
		return fmt.Errorf("本地快照空间不足：需要%d字节，可用%d字节", need, free)
	}
	return ctx.Err()
}

type localSpaceMonitor struct {
	repository string
	last       time.Time
	bytes      int64
}

func (s *localSpaceMonitor) check(ctx context.Context, written int64, force bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.bytes += written
	if !force && s.bytes < 8<<20 && time.Since(s.last) < 250*time.Millisecond {
		return nil
	}
	free, total, err := localVolumeSpace(s.repository)
	if err != nil {
		return err
	}
	if free < localReserveSpace(total) {
		return errors.New("本地快照剩余空间低于保留余量")
	}
	s.bytes, s.last = 0, time.Now()
	return ctx.Err()
}
