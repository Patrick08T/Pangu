package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

// runLocalGitOutput preserves the bounded raw-output contract used by artifact
// readers, while owning the child tree and durable operation through its stop.
func runLocalGitOutput(ctx context.Context, directory string, limit int, args ...string) (data []byte, truncated bool, result error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		result = finish(result)
		if result != nil {
			data, truncated = nil, false
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := localGitCommand(ctx, directory, args...)
	out, stderr := &diagnostic.Buffer{Limit: limit}, &diagnostic.Buffer{Limit: 4096}
	cmd.Stdout, cmd.Stderr = out, stderr
	err = recordLocalGitOperationError(ctx, runManagedLocalGit(cmd))
	if cause := context.Cause(ctx); cause != nil {
		err = errors.Join(cause, err)
	}
	if err != nil {
		message, _ := diagnostic.Text(strings.ReplaceAll(stderr.Data.String(), directory, "[工作目录]"), 4096)
		return nil, false, fmt.Errorf("Git 操作失败: %w；%s", err, message)
	}
	return out.Data.Bytes(), out.Truncated, nil
}
