package agent

import (
	"context"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/workflow"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type processExitError int

func (e processExitError) Error() string { return fmt.Sprintf("进程退出码 %d", e) }
func (e processExitError) ExitCode() int { return int(e) }

func Verify(ctx context.Context, directory string, args []string, scope string) (workflow.CommandEvidence, error) {
	evidence := workflow.CommandEvidence{ID: "project-verification", ExitCode: -1}
	if len(args) == 0 || !filepath.IsAbs(args[0]) || !filepath.IsAbs(directory) {
		return evidence, &workflow.Failure{Kind: "parameter", Cause: errors.New("验证需要管理员登记的绝对程序路径和工作区")}
	}
	home, err := os.MkdirTemp("", "pangu-verification-")
	if err != nil {
		return evidence, err
	}
	defer os.RemoveAll(home)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir, cmd.Stdin = directory, strings.NewReader("")
	cmd.Env = verificationEnvironment(home)
	cmd.WaitDelay = time.Second
	stdout, stderr := &diagnostic.Buffer{Limit: 512 * 1024}, &diagnostic.Buffer{Limit: 512 * 1024}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err = runCodexProcess(ctx, cmd, scope)
	evidence.Command, _ = diagnostic.Text(strings.Join(args, " "), 1024)
	evidence.Output, evidence.Truncated = diagnostic.Text(stdout.Data.String()+stderr.Data.String(), 4096)
	evidence.Truncated = evidence.Truncated || stdout.Truncated || stderr.Truncated
	if err == nil {
		evidence.ExitCode = 0
		return evidence, nil
	}
	var exit interface{ ExitCode() int }
	if errors.As(err, &exit) {
		evidence.ExitCode = exit.ExitCode()
	}
	return evidence, &workflow.Failure{Kind: "verification", Cause: err}
}

func verificationEnvironment(home string) []string {
	values := []string{"HOME=" + home, "USERPROFILE=" + home}
	for _, key := range []string{"PATH", "SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "SYSTEMDRIVE", "TEMP", "TMP", "GOCACHE", "GOMODCACHE"} {
		if value, ok := os.LookupEnv(key); ok {
			values = append(values, key+"="+value)
		}
	}
	return values
}
