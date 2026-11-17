package workspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

func importLocalFiles(ctx context.Context, repository, root, parent string, manifest localManifest) (commit string, err error) {
	ctx, finish, err := beginLocalGitOperation(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		err = finish(err)
		if err != nil {
			commit = ""
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	if err = localCommitExists(ctx, repository, parent); err != nil {
		return "", err
	}
	directory, info, err := openLocalRoot(root)
	if err != nil {
		return "", err
	}
	defer directory.Close()
	if manifest.identity == nil || !samePath(root, manifest.root) || !os.SameFile(info, manifest.identity) {
		return "", errors.New("本地快照清单根身份不匹配")
	}
	if err = validateLocalManifest(ctx, manifest); err != nil {
		return "", err
	}
	temporary := "refs/pangu-import/" + uuid.NewString()
	final, published := "refs/heads/local-"+uuid.NewString(), ""
	defer func() {
		err = cleanupLocalImportContext(ctx, repository, temporary, final, published, err)
		if err != nil {
			commit = ""
		}
	}()
	if err = runLocalImport(ctx, repository, root, parent, temporary, directory, manifest); err != nil {
		return "", err
	}
	commit, err = runLocalGit(ctx, repository, "rev-parse", "--verify", temporary+"^{commit}")
	if err != nil {
		return "", err
	}
	if err = inspectLocalRepository(ctx, repository, commit); err != nil {
		return "", err
	}
	if _, err = runLocalGit(ctx, repository, localObjectCheckArgs(commit)...); err != nil {
		return "", err
	}
	if err = validateLocalRoot(root, directory, manifest.identity); err != nil {
		return "", err
	}
	if _, err = runLocalGit(ctx, repository, "update-ref", "-d", temporary, commit); err != nil {
		return "", err
	}
	published = commit
	_, err = runLocalGit(ctx, repository, "update-ref", final, commit, strings.Repeat("0", len(commit)))
	if err != nil {
		return "", err
	}
	return commit, nil
}

func runLocalImport(ctx context.Context, repository, root, parent, ref string, directory *os.Root, m localManifest) error {
	return runLocalImportWithGuard(ctx, repository, root, parent, ref, directory, m, localVolumeSpace, localGitCommand)
}

func writeLocalImport(ctx context.Context, input io.Writer, repository, root, parent, ref string, directory *os.Root, m localManifest) error {
	space := &localSpaceMonitor{repository: repository}
	if err := space.check(ctx, 0, true); err != nil {
		return err
	}
	out := bufio.NewWriterSize(input, 64<<10)
	message := "Pangu local P4 snapshot\n"
	if _, err := fmt.Fprintf(out, "commit %s\ncommitter Pangu <pangu@localhost> %d +0000\ndata %d\n%sfrom %s\ndeleteall\n", ref, time.Now().Unix(), len(message), message, parent); err != nil {
		return err
	}
	buffer := make([]byte, 64<<10)
	for _, file := range m.Files {
		if err := writeLocalFile(ctx, out, directory, root, file, buffer, space); err != nil {
			return err
		}
	}
	if err := validateLocalRoot(root, directory, m.identity); err != nil {
		return err
	}
	if err := space.check(ctx, 0, true); err != nil {
		return err
	}
	if _, err := io.WriteString(out, "\ndone\n"); err != nil {
		return err
	}
	return out.Flush()
}
