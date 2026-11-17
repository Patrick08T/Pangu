package workspace

import "golang.org/x/sys/windows"

func localVolumeSpace(path string) (free, total uint64, err error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var unused uint64
	err = windows.GetDiskFreeSpaceEx(name, &free, &total, &unused)
	return free, total, err
}
