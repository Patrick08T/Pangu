package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
)

func localGitOperationRoot() (*os.Root, string, string, error) {
	path, err := workspaceRoot()
	if err != nil {
		return nil, "", "", err
	}
	if WorkerID() == "" {
		return nil, "", "", errors.New("本地 Git 操作缺少 Worker 身份")
	}
	root, err := os.OpenRoot(path)
	name := fmt.Sprintf(".pangu-local-git-%x.guard", sha256.Sum256([]byte(WorkerID())))
	return root, path, name, err
}

func checkLocalGitOperationFile() error {
	root, _, name, err := localGitOperationRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	_, err = root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return errors.Join(errLocalGitOperationBlocked, err)
}

func createLocalGitOperation() (*localGitOperation, error) {
	root, path, name, err := localGitOperationRoot()
	if err != nil {
		return nil, err
	}
	owner := &localGitOperation{root: root, path: path, name: name}
	owner.rootInfo, err = root.Stat(".")
	if err == nil {
		owner.payload, err = json.Marshal(struct {
			Version int    `json:"version"`
			ID      string `json:"id"`
			Worker  string `json:"worker"`
			Started string `json:"started"`
		}{1, uuid.NewString(), WorkerID(), time.Now().UTC().Format(time.RFC3339Nano)})
	}
	if err == nil {
		err = owner.persist()
	}
	if err != nil {
		return nil, errors.Join(errLocalGitOperationBlocked, err, root.Close())
	}
	return owner, nil
}

func (owner *localGitOperation) persist() error {
	file, err := owner.root.OpenFile(owner.name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, writeErr := file.Write(owner.payload)
	if n != len(owner.payload) && writeErr == nil {
		writeErr = io.ErrShortWrite
	}
	// A partial/unsynced marker is deliberately retained: never start Git after it.
	syncErr := file.Sync()
	owner.guardInfo, err = file.Stat()
	return errors.Join(writeErr, syncErr, err, file.Close())
}

func (owner *localGitOperation) checkMarker() error {
	info, err := os.Stat(owner.path)
	if err != nil || !os.SameFile(info, owner.rootInfo) {
		return errors.Join(errLocalGitOperationBlocked, err)
	}
	info, err = owner.root.Lstat(owner.name)
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(info, owner.guardInfo) {
		return errors.Join(errLocalGitOperationBlocked, err)
	}
	file, err := owner.root.Open(owner.name)
	if err != nil {
		return errors.Join(errLocalGitOperationBlocked, err)
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !os.SameFile(info, owner.guardInfo) {
		return errors.Join(errLocalGitOperationBlocked, err)
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || !bytes.Equal(data, owner.payload) {
		return errors.Join(errLocalGitOperationBlocked, err)
	}
	return nil
}

func (owner *localGitOperation) finish(cause error) error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer owner.root.Close()
	owner.closed = true
	if owner.unknown != nil {
		return errors.Join(cause, owner.unknown, errLocalGitOperationBlocked)
	}
	if err := owner.checkMarker(); err != nil {
		return errors.Join(cause, err)
	}
	if err := owner.root.Remove(owner.name); err != nil {
		return errors.Join(cause, errLocalGitOperationBlocked, err)
	}
	return cause
}
