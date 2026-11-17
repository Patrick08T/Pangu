package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
)

func validateLocalManifest(ctx context.Context, m localManifest) error {
	if m.Missing < 0 || m.Missing > localLimits.entries || len(m.Files) > localLimits.entries-m.Missing {
		return errors.New("本地快照清单数量无效")
	}
	seen, total, paths, largest := make(map[string]bool), int64(0), int64(0), int64(0)
	for _, file := range m.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := validateLocalName(file.Name); err != nil {
			return err
		}
		key := localNameKey(file.Name)
		if seen[key] || file.identity == nil || !file.Mode.IsRegular() || file.Size < 0 || file.Size > math.MaxInt64-total {
			return errors.New("本地快照清单身份或大小无效")
		}
		seen[key] = true
		total, paths, largest = total+file.Size, paths+int64(len(file.Name)), max(largest, file.Size)
		if paths > int64(localLimits.paths) {
			return errors.New("本地快照清单超过路径预算")
		}
	}
	if total != m.TotalBytes || paths != m.PathBytes || largest != m.Largest {
		return errors.New("本地快照清单统计不匹配")
	}
	return nil
}

// Git accepts C-style octal bytes, not Go's Unicode escape notation.
func quoteLocalGitPath(name string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, b := range []byte(name) {
		if b >= 127 || b < 32 || b == '"' || b == '\\' {
			out.WriteByte('\\')
			out.WriteByte('0' + b/64)
			out.WriteByte('0' + b/8%8)
			out.WriteByte('0' + b%8)
		} else {
			out.WriteByte(b)
		}
	}
	out.WriteByte('"')
	return out.String()
}

func matchingLocalFile(info fs.FileInfo, file localFileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Size() == file.Size && info.Mode() == file.Mode && info.ModTime().Equal(file.ModTime) && os.SameFile(info, file.identity)
}

func openLocalFile(ctx context.Context, directory *os.Root, root string, info localFileInfo) (*os.File, error) {
	current, err := validateLocalChain(ctx, directory, root, info.Name)
	if err != nil {
		return nil, err
	}
	if !matchingLocalFile(current, info) {
		return nil, errors.New("本地快照源文件在清单后发生变化")
	}
	file, err := directory.Open(filepath.FromSlash(info.Name))
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !matchingLocalFile(opened, info) {
		file.Close()
		return nil, errors.New("本地快照打开文件身份变化")
	}
	return file, nil
}

func writeLocalFile(ctx context.Context, out io.Writer, directory *os.Root, root string, info localFileInfo, buffer []byte, space *localSpaceMonitor) error {
	if err := space.check(ctx, 0, false); err != nil {
		return err
	}
	file, err := openLocalFile(ctx, directory, root, info)
	if err != nil {
		return err
	}
	defer file.Close()
	mode := "100644"
	if info.Mode.Perm()&0111 != 0 {
		mode = "100755"
	}
	if _, err := fmt.Fprintf(out, "M %s inline %s\ndata %d\n", mode, quoteLocalGitPath(info.Name), info.Size); err != nil {
		return err
	}
	if err := copyLocalBytes(ctx, out, file, info.Size, buffer, space); err != nil {
		return err
	}
	opened, err := file.Stat()
	current, pathErr := validateLocalChain(ctx, directory, root, info.Name)
	if err != nil || pathErr != nil || !matchingLocalFile(opened, info) || !matchingLocalFile(current, info) {
		return errors.New("本地快照读取期间源文件发生变化")
	}
	_, err = io.WriteString(out, "\n")
	return err
}

func copyLocalBytes(ctx context.Context, out io.Writer, file *os.File, size int64, buffer []byte, space *localSpaceMonitor) error {
	for remaining := size; remaining > 0; {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := file.Read(buffer[:min(int64(len(buffer)), remaining)])
		if n > 0 {
			if err := space.check(ctx, int64(n), false); err != nil {
				return err
			}
			written, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
			remaining -= int64(n)
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	var extra [1]byte
	if n, err := file.Read(extra[:]); n != 0 || err != io.EOF {
		return errors.New("本地快照文件大小在读取期间变化")
	}
	return ctx.Err()
}
