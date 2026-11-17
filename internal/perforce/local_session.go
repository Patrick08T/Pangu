package perforce

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type localCommandSession struct {
	config                    Config
	client, directory, binary string
	env                       []string
	info                      []Record
}

func (s Snapshot) newLocalSession() (*localCommandSession, error) {
	if err := s.Config.Validate(); err != nil {
		return nil, err
	}
	if !identifier.MatchString(s.Client) || s.Client != s.Config.Client || !filepath.IsAbs(s.LocalRoot) {
		return nil, errors.New("Perforce 本地目录或 Client 绑定无效")
	}
	session := &localCommandSession{config: s.Config, client: s.Client, directory: filepath.Clean(s.LocalRoot), env: environment()}
	binary := "p4"
	for _, entry := range session.env {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "PANGU_P4_BINARY") && value != "" {
			binary = value
		}
	}
	resolved, err := exec.LookPath(binary)
	if err != nil {
		return nil, err
	}
	session.binary, err = filepath.Abs(resolved)
	return session, err
}

func (s *localCommandSession) process(ctx context.Context, args ...string) (*exec.Cmd, error) {
	charset := "none"
	if len(args) != 1 || args[0] != "info" {
		var err error
		charset, err = commandCharset(s.info, s.config.Scope, "", args)
		if err != nil {
			return nil, err
		}
	}
	options := []string{"-Mj", "-ztag", "-C", charset, "-v", "net.maxwait=30", "-p", s.config.Port, "-u", s.config.User, "-c", s.client, "-d", s.directory}
	if charset == "utf8" {
		options = append(options, "-Q", "utf8")
	}
	stdin, commandArgs, err := unicodeArguments("", args)
	if err != nil {
		return nil, err
	}
	if stdin != "" {
		options = append(options, "-x", "-")
	}
	cmd := exec.CommandContext(ctx, s.binary, append(options, commandArgs...)...)
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.WaitDelay = s.directory, s.env, strings.NewReader(stdin), time.Second
	return cmd, nil
}

// File batches use stdin even for ASCII, avoiding the Windows argv length limit.
func (s *localCommandSession) streamFiles(ctx context.Context, limits localFileLimits, args, files []string) error {
	if _, err := commandCharset(s.info, s.config.Scope, "", files); err != nil {
		return err
	}
	return runLocalStream(ctx, limits, func(Record) error { return ctx.Err() }, func(child context.Context) (*exec.Cmd, error) {
		cmd, err := s.process(child, args...)
		if err != nil {
			return nil, err
		}
		cmd.Args = append([]string{cmd.Args[0], "-x", "-"}, cmd.Args[1:]...)
		cmd.Stdin = strings.NewReader(strings.Join(files, "\n") + "\n")
		return cmd, nil
	})
}
