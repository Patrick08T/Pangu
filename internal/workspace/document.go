package workspace

import (
	"bytes"
	"context"
	"errors"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

func Document(ctx context.Context, run, name string) ([]byte, string, error) {
	if !utf8.ValidString(name) || path.IsAbs(name) || path.Clean(name) != name || name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.ContainsAny(name, ":\\\x00\r\n") {
		return nil, "", errors.New("资料路径必须为规范的项目内相对路径")
	}
	for _, part := range strings.Split(strings.ToLower(name), "/") {
		if part == ".git" || part == ".pangu" || part == ".aws" || strings.HasPrefix(part, ".env") || strings.HasSuffix(part, ".pem") || strings.HasSuffix(part, ".key") || part == "credentials" || part == "id_rsa" || part == "id_ed25519" {
			return nil, "", errors.New("拒绝读取凭据或平台文件")
		}
	}
	set, err := Files(ctx, run)
	if err != nil {
		return nil, "", err
	}
	entry, err := git(ctx, set.repository, "ls-tree", "-z", set.Base, "--", name)
	if err != nil {
		return nil, "", err
	}
	fields := strings.SplitN(strings.TrimSuffix(entry, "\x00"), "\t", 2)
	if len(fields) != 2 || fields[1] != name {
		return nil, "", errors.New("授权资料在原始基准中不存在")
	}
	meta := strings.Fields(fields[0])
	if len(meta) != 3 || meta[1] != "blob" || (meta[0] != "100644" && meta[0] != "100755") {
		return nil, "", errors.New("资料必须为普通文件，不允许符号链接")
	}
	size, err := set.size(ctx, meta[2])
	if err != nil || size > 64*1024 {
		return nil, "", errors.New("资料不可读或超过 64 KiB")
	}
	data, err := set.Blob(ctx, meta[2])
	if err != nil {
		return nil, "", err
	}
	if !utf8.Valid(data) || bytes.ContainsRune(data, 0) || diagnostic.Sensitive(data) {
		return nil, "", errors.New("资料必须为无凭据的 UTF-8 文本")
	}
	return data, set.Base, nil
}
