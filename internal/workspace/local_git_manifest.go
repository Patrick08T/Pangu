package workspace

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func localNameKey(name string) string {
	return strings.Map(func(r rune) rune {
		lowest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			lowest = min(lowest, next)
		}
		return lowest
	}, name)
}

func validateLocalName(name string) error {
	if name == "" || !utf8.ValidString(name) || !filepath.IsLocal(name) || path.Clean(name) != name || len(name) > localLimits.record {
		return errors.New("本地快照路径无效或越界")
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || strings.ContainsRune("\\:<>\"|?*", r) {
			return errors.New("本地快照含歧义路径")
		}
	}
	for _, part := range strings.Split(name, "/") {
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		reserved := base == "CON" || base == "PRN" || base == "AUX" || base == "NUL"
		if (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && len(base) > 3 {
			suffix := base[3:]
			reserved = reserved || utf8.RuneCountInString(suffix) == 1 && strings.Contains("123456789¹²³", suffix)
		}
		if part == "." || part == ".." || strings.EqualFold(part, ".git") || strings.HasPrefix(strings.ToLower(part), "git~") || strings.Contains(part, "...") || strings.TrimRight(part, ". ") != part || reserved {
			return errors.New("本地快照含元数据或歧义路径")
		}
	}
	return nil
}

func openLocalRoot(root string) (*os.Root, fs.FileInfo, error) {
	actual, err := existingPath(root)
	if err != nil || !filepath.IsAbs(root) || !samePath(actual, filepath.Clean(root)) {
		return nil, nil, errors.New("本地快照根目录被替换或跳转")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeType != os.ModeDir {
		return nil, nil, errors.New("本地快照根目录不是普通目录")
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, err
	}
	if _, err := directory.Lstat(".git"); !os.IsNotExist(err) {
		directory.Close()
		return nil, nil, errors.New("本地P4根目录不能包含Git元数据")
	}
	return directory, info, nil
}

func validateLocalChain(ctx context.Context, directory *os.Root, root, name string) (fs.FileInfo, error) {
	parts, relative := strings.Split(name, "/"), ""
	var info fs.FileInfo
	for i, part := range parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		relative = filepath.Join(relative, part)
		var err error
		info, err = directory.Lstat(relative)
		if err != nil {
			return nil, err
		}
		if i < len(parts)-1 && !info.IsDir() || info.Mode()&os.ModeType != 0 && info.Mode()&os.ModeType != os.ModeDir {
			return nil, errors.New("本地快照拒绝链接或特殊文件")
		}
		actual, err := existingPath(filepath.Join(root, relative))
		if err != nil || !samePath(actual, filepath.Join(root, relative)) {
			return nil, errors.New("本地快照拒绝目录别名或跳转")
		}
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("本地快照仅接受普通文件")
	}
	return info, nil
}

func inspectLocalFiles(ctx context.Context, root string, names []string) (localManifest, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return localManifest{}, err
	}
	if len(names) > localLimits.entries {
		return localManifest{}, errors.New("本地快照超过文件数量预算")
	}
	directory, rootInfo, err := openLocalRoot(root)
	if err != nil {
		return localManifest{}, err
	}
	defer directory.Close()
	m := localManifest{root: filepath.Clean(root), identity: rootInfo}
	seen, paths := make(map[string]bool), 0
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return localManifest{}, err
		}
		if err := validateLocalName(name); err != nil {
			return localManifest{}, err
		}
		key := localNameKey(name)
		if seen[key] || len(name) > localLimits.paths-paths {
			return localManifest{}, errors.New("本地快照路径重复或超过字节预算")
		}
		seen[key], paths = true, paths+len(name)
		info, err := validateLocalChain(ctx, directory, root, name)
		if os.IsNotExist(err) {
			m.Missing++
			continue
		}
		if err != nil {
			return localManifest{}, err
		}
		if info.Size() < 0 || info.Size() > math.MaxInt64-m.TotalBytes {
			return localManifest{}, errors.New("本地快照大小溢出")
		}
		m.Files = append(m.Files, localFileInfo{Name: name, Size: info.Size(), Mode: info.Mode(), ModTime: info.ModTime(), identity: info})
		m.TotalBytes += info.Size()
		m.PathBytes += int64(len(name))
		m.Largest = max(m.Largest, info.Size())
	}
	if err := validateLocalRoot(root, directory, rootInfo); err != nil {
		return localManifest{}, err
	}
	return m, ctx.Err()
}

func validateLocalRoot(root string, directory *os.Root, original fs.FileInfo) error {
	info, err := directory.Stat(".")
	current, pathErr := os.Lstat(root)
	actual, resolveErr := existingPath(root)
	if err != nil || pathErr != nil || resolveErr != nil || !os.SameFile(original, info) || !os.SameFile(original, current) || !samePath(actual, filepath.Clean(root)) {
		return errors.New("本地快照根目录身份变化")
	}
	return nil
}
