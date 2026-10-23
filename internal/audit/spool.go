package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type pending struct {
	Namespace string `json:"namespace"`
	Entry     Entry  `json:"entry"`
}

func spoolRoot(namespace string) (*os.Root, error) {
	directory := os.Getenv("PANGU_AUDIT_SPOOL_ROOT")
	if directory == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		directory = filepath.Join(base, "Pangu", "audit")
	}
	if !filepath.IsAbs(directory) {
		return nil, errors.New("审计补录目录必须为绝对路径")
	}
	digest := sha256.Sum256([]byte(namespace))
	directory = filepath.Join(directory, hex.EncodeToString(digest[:]))
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	return os.OpenRoot(directory)
}

// 意图和结果各自原子落盘；重复回放不改变既有审计终态。
func Queue(entry Entry, status int) error {
	if _, err := uuid.Parse(entry.ID); err != nil {
		return err
	}
	if entry.Namespace == "" || entry.CreatedAt.IsZero() {
		return errors.New("审计元数据不完整")
	}
	phase := "intent"
	entry.Outcome = "pending"
	if status != 0 {
		if status < 100 || status > 599 {
			return errors.New("审计响应码无效")
		}
		phase = "result"
		entry.Outcome = "succeeded"
		if status >= 500 {
			entry.Outcome = "failed"
		} else if status >= 400 {
			entry.Outcome = "rejected"
		}
		now := time.Now().UTC()
		entry.HTTPStatus, entry.FinishedAt = &status, &now
	}
	data, err := json.Marshal(pending{Namespace: entry.Namespace, Entry: entry})
	if err != nil {
		return err
	}
	if len(data) > 16384 {
		return errors.New("审计补录元数据超限")
	}
	root, err := spoolRoot(entry.Namespace)
	if err != nil {
		return err
	}
	defer root.Close()
	name := entry.ID + "." + phase + ".json"
	if old, err := root.ReadFile(name); err == nil {
		if bytes.Equal(old, data) {
			return nil
		}
		return errors.New("审计补录文件已存在且内容不同")
	}
	return storePending(root, name, data)
}

func storePending(root *os.Root, name string, data []byte) error {
	temporary := ".pending-" + uuid.NewString()
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return root.Rename(temporary, name)
}
