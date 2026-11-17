package perforce

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

func localSegmentsContext(ctx context.Context, path string, encoded bool) ([]string, error) {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if encoded {
			var err error
			part, err = url.PathUnescape(part)
			if err != nil {
				return nil, err
			}
		}
		if part == "" || part == "." || part == ".." || strings.Contains(part, "...") ||
			strings.ContainsAny(part, "/\\:\x00\r\n\t") || strings.TrimRight(part, ". ") != part ||
			!utf8.ValidString(part) || strings.ContainsRune(part, '\ufffd') || !filepath.IsLocal(part) {
			return nil, errors.New("P4 文件路径存在编码、越界或歧义")
		}
		for _, r := range part {
			if unicode.IsControl(r) {
				return nil, errors.New("P4 文件路径含控制字符")
			}
		}
		parts[i] = part
	}
	return parts, nil
}

func (s Snapshot) normalizedLocalNameContext(ctx context.Context, name string) (string, error) {
	if !filepath.IsAbs(s.LocalRoot) {
		return "", errors.New("P4 本地根目录无效")
	}
	if strings.HasPrefix(name, "//") {
		prefix := "//" + s.Client + "/"
		if !strings.HasPrefix(name, prefix) {
			return "", errors.New("P4 文件路径命名空间不匹配")
		}
		parts, err := localSegmentsContext(ctx, strings.TrimPrefix(name, prefix), true)
		if err != nil {
			return "", err
		}
		name = filepath.Join(s.LocalRoot, filepath.Join(parts...))
	} else {
		path := filepath.ToSlash(name)
		volume := filepath.ToSlash(filepath.VolumeName(name))
		path = strings.TrimPrefix(strings.TrimPrefix(path, volume), "/")
		if _, err := localSegmentsContext(ctx, path, false); err != nil {
			return "", err
		}
		if !filepath.IsAbs(name) {
			if !filepath.IsLocal(name) {
				return "", errors.New("P4 文件路径不是本地路径")
			}
			name = filepath.Join(s.LocalRoot, name)
		}
	}
	root := filepath.Join(s.LocalRoot, filepath.FromSlash(s.Config.Scope))
	rel, err := filepath.Rel(root, name)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", errors.New("P4 文件路径超出绑定的项目目录")
	}
	return filepath.ToSlash(rel), nil
}

func (s Snapshot) inventoryName(ctx context.Context, raw string) (string, bool, error) {
	name, err := s.normalizedLocalNameContext(ctx, raw)
	if err != nil {
		return "", false, err
	}
	parts := strings.Split(name, "/")
	for i, part := range parts {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		if !strings.EqualFold(part, ".git") {
			continue
		}
		if i == 0 {
			return "", false, errors.New("P4 项目根目录不能包含 Git 元数据")
		}
		marker := filepath.Join(s.LocalRoot, filepath.FromSlash(s.Config.Scope), filepath.Join(parts[:i+1]...))
		if err := localAncestors(ctx, marker); err != nil {
			return "", false, err
		}
		return name, true, nil
	}
	return name, false, nil
}

func localAncestors(ctx context.Context, name string) error {
	volume := filepath.VolumeName(name)
	current := volume + string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(name, volume), string(filepath.Separator)), string(filepath.Separator))
	for i, part := range parts {
		if err := ctx.Err(); err != nil {
			return err
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeType != 0 && !info.IsDir() {
			return errors.New("P4 文件祖先不能包含链接或特殊文件")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return errors.New("P4 文件祖先不是目录")
		}
	}
	return nil
}

func (s Snapshot) checkInventoryRoot(ctx context.Context) error {
	root := filepath.Join(s.LocalRoot, filepath.FromSlash(s.Config.Scope))
	if err := localAncestors(ctx, root); err != nil {
		return err
	}
	dir, err := os.Open(root)
	if err != nil {
		return err
	}
	defer dir.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := dir.ReadDir(128)
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), ".git") {
				return errors.New("P4 项目根目录不能包含 Git 元数据")
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
