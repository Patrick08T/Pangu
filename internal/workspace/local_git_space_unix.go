//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package workspace

import (
	"errors"
	"golang.org/x/sys/unix"
	"math"
)

func localVolumeSpace(path string) (free, total uint64, err error) {
	var stat unix.Statfs_t
	if err = unix.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	block := uint64(stat.Bsize)
	if block == 0 || uint64(stat.Bavail) > math.MaxUint64/block || uint64(stat.Blocks) > math.MaxUint64/block {
		return 0, 0, errors.New("卷容量溢出")
	}
	return uint64(stat.Bavail) * block, uint64(stat.Blocks) * block, nil
}
