//go:build !windows && !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package workspace

import "errors"

func localVolumeSpace(path string) (free, total uint64, err error) {
	return 0, 0, errors.New("当前平台不支持本地快照卷容量检测")
}
