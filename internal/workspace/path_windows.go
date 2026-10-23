package workspace

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// Windows junction 在 Go 1.23+ 中不是 ModeSymlink，必须读取内核解析后的路径。
func existingPath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	buffer := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(windows.Handle(file.Fd()), &buffer[0], uint32(len(buffer)), 0)
	if err != nil {
		return "", err
	}
	if n >= uint32(len(buffer)) {
		return "", os.ErrInvalid
	}
	value := windows.UTF16ToString(buffer[:n])
	if strings.HasPrefix(value, `\\?\UNC\`) {
		return `\\` + strings.TrimPrefix(value, `\\?\UNC\`), nil
	}
	return strings.TrimPrefix(value, `\\?\`), nil
}
