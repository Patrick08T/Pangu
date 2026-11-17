package workspace

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func inspectLocalGitMetadata(ctx context.Context, source string, visited *int, usedPaths uint64, limits localGitLimits) (localObjectStore, error) {
	name := filepath.Join(source, ".git")
	identity, err := localObjectDirectory(ctx, name)
	if err != nil {
		return localObjectStore{}, err
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return localObjectStore{}, err
	}
	defer root.Close()
	store := localObjectStore{root: name, identity: identity, files: map[string]fs.FileInfo{}, metadataScan: true}
	limits.paths -= int(usedPaths)
	if err := scanLocalObjectDirectory(ctx, root, ".", 0, visited, &store, limits); err != nil {
		return localObjectStore{}, err
	}
	for config, info := range store.files {
		if err := ctx.Err(); err != nil {
			return localObjectStore{}, err
		}
		if strings.EqualFold(config, "config") || strings.EqualFold(config, "config.worktree") {
			if err := inspectLocalGitConfig(ctx, root, config, info, limits.record); err != nil {
				return localObjectStore{}, err
			}
		}
	}
	current, err := localObjectDirectory(ctx, name)
	if err != nil {
		return localObjectStore{}, err
	}
	if !os.SameFile(identity, current) {
		return localObjectStore{}, errors.New("本地Git元数据目录已替换")
	}
	return store, ctx.Err()
}

func inspectLocalGitConfig(ctx context.Context, root *os.Root, name string, before fs.FileInfo, budget int) error {
	if before.Size() > int64(budget) {
		return errors.New("本地Git配置超过读取预算")
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(before, info) || !info.Mode().IsRegular() {
		return errors.New("本地Git配置身份已变化")
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(budget)+1))
	if err != nil {
		return err
	}
	if len(body) > budget || !utf8.Valid(body) || strings.ContainsRune(string(body), 0) {
		return errors.New("本地Git配置超过预算或无法安全解析")
	}
	if err := validateLocalGitConfig(ctx, string(body)); err != nil {
		return err
	}
	after, err := file.Stat()
	if err != nil {
		return err
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return errors.New("本地Git配置在检查期间变化")
	}
	return ctx.Err()
}

func validateLocalGitConfig(ctx context.Context, body string) error {
	section := ""
	for _, raw := range strings.Split(body, "\n") {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasSuffix(line, "\\") {
			return errors.New("本地Git配置不接受续行歧义")
		}
		if strings.HasPrefix(line, "[") {
			var err error
			section, err = localGitConfigSection(line)
			if err != nil {
				return err
			}
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok || !localGitConfigKeyAllowed(section, strings.ToLower(strings.TrimSpace(key))) {
			return errors.New("本地Git配置包含外借、执行或未知选项")
		}
	}
	return nil
}

func localGitConfigSection(line string) (string, error) {
	end := strings.IndexByte(line, ']')
	if end < 0 {
		return "", errors.New("本地Git配置节未闭合")
	}
	tail := strings.TrimSpace(line[end+1:])
	if tail != "" && !strings.HasPrefix(tail, "#") && !strings.HasPrefix(tail, ";") {
		return "", errors.New("本地Git配置节存在歧义")
	}
	parts := strings.Fields(strings.ToLower(line[1:end]))
	if len(parts) == 0 {
		return "", errors.New("本地Git配置节为空")
	}
	section := parts[0]
	if section != "core" && section != "user" && section != "branch" && section != "remote" && section != "init" {
		return "", errors.New("本地Git配置包含外借或未知配置节")
	}
	if len(parts) > 1 && section != "branch" && section != "remote" {
		return "", errors.New("本地Git配置子节存在歧义")
	}
	return section, nil
}

func localGitConfigKeyAllowed(section, key string) bool {
	allowed := map[string]string{
		"core":   " repositoryformatversion filemode bare logallrefupdates symlinks ignorecase longpaths ",
		"user":   " name email ",
		"branch": " remote merge ",
		"remote": " url fetch tagopt ",
		"init":   " defaultbranch ",
	}
	return key != "" && strings.Contains(allowed[section], " "+key+" ")
}
