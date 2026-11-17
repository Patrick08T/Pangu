package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/perforce"
	"github.com/jackc/pgx/v5"
)

type P4Delivery struct {
	Dispatch        bool     `json:"-"`
	Baseline        int64    `json:"baseline"`
	Client          string   `json:"client"`
	Changelist      int64    `json:"changelist,omitempty"`
	Checkpoint      string   `json:"checkpoint"`
	Status          string   `json:"status"`
	Error           string   `json:"error,omitempty"`
	LocalDirectory  string   `json:"local_directory,omitempty"`
	ExistingChanges []string `json:"existing_changelists,omitempty"`
}

func PerforceDelivery(ctx context.Context, id string) (*P4Delivery, error) {
	conn, err := database.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	return readP4Delivery(ctx, conn, id)
}

func readP4Delivery(ctx context.Context, conn *pgx.Conn, id string) (*P4Delivery, error) {
	out := &P4Delivery{}
	err := conn.QueryRow(ctx, `SELECT s.baseline,coalesce(d.client,s.client),coalesce(d.changelist,s.work_change,0),coalesce(d.checkpoint,''),coalesce(d.status,'available'),coalesce(d.error,''),s.local_path,s.existing_changes FROM pangu.perforce_sources s LEFT JOIN pangu.perforce_deliveries d USING(run_id) WHERE s.run_id=$1`, id).
		Scan(&out.Baseline, &out.Client, &out.Changelist, &out.Checkpoint, &out.Status, &out.Error, &out.LocalDirectory, &out.ExistingChanges)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return out, err
}

func DeliverPerforce(ctx context.Context, id string) (*P4Delivery, error) {
	conn, err := lockRun(ctx, id)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	record, err := readRun(ctx, conn, id)
	if err != nil {
		return nil, err
	}
	if record.ExpiredAt != nil {
		return nil, ErrExpired
	}
	s, err := readPerforce(ctx, conn, id)
	if err != nil {
		return nil, errors.New("运行没有 Perforce 来源")
	}
	if s.LocalRoot != "" {
		return deliverLocalPerforce(ctx, conn, record, s)
	}
	set, err := filesBetween(ctx, record, s.Commit)
	if err != nil {
		return nil, err
	}
	s.Client += "-review"
	s.ForDelivery = true
	_, err = conn.Exec(ctx, `INSERT INTO pangu.perforce_deliveries(run_id,checkpoint,client,status) VALUES($1,$2,$3,'preparing') ON CONFLICT(run_id) DO NOTHING`, id, record.HeadCommit, s.Client)
	if err != nil {
		return nil, err
	}
	delivery, err := readP4Delivery(ctx, conn, id)
	if err != nil {
		return nil, err
	}
	if delivery.Checkpoint != record.HeadCommit {
		return nil, errors.New("交付检查点已经固定，后续修改需要新的运行")
	}
	if delivery.Status == "shelved" || delivery.Status == "empty" {
		return delivery, nil
	}
	status := "shelved"
	if len(set.Changes) == 0 {
		status = "empty"
	} else {
		err = exportPerforce(ctx, conn, record, s, set)
	}
	message := ""
	if err != nil {
		status = "failed"
		message, _ = diagnostic.Text(err.Error(), 4096)
	}
	_, saveErr := conn.Exec(ctx, `UPDATE pangu.perforce_deliveries SET status=$2,error=$3,updated_at=now() WHERE run_id=$1`, id, status, message)
	if saveErr != nil {
		return nil, saveErr
	}
	return readP4Delivery(ctx, conn, id)
}

func exportPerforce(ctx context.Context, conn *pgx.Conn, record *Run, s *perforce.Snapshot, set *Fileset) error {
	for _, change := range set.Changes {
		if err := validP4Change(change); err != nil {
			return err
		}
	}
	root, err := s.Prepare(ctx, filepath.Join(record.Directory, "p4-delivery"))
	if err != nil {
		return err
	}
	if err = inspectPerforce(root); err != nil {
		return err
	}
	change, err := s.PendingChange(ctx)
	if err != nil {
		return err
	}
	if _, err = conn.Exec(ctx, `UPDATE pangu.perforce_deliveries SET changelist=$2,status='preparing',error='',updated_at=now() WHERE run_id=$1`, record.ID, change); err != nil {
		return err
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer directory.Close()
	paths := []string{}
	for _, file := range set.Changes {
		if err = exportP4File(ctx, directory, set, file); err != nil {
			return err
		}
		paths = append(paths, file.Path)
	}
	return s.Reconcile(ctx, change, paths)
}

func validP4Change(change Change) error {
	if !filepath.IsLocal(filepath.FromSlash(change.Path)) || strings.ContainsAny(change.Path, "\\\r\n\x00") {
		return errors.New("交付文件路径不安全")
	}
	for _, part := range strings.Split(change.Path, "/") {
		if strings.EqualFold(part, ".git") || strings.Contains(part, "...") {
			return errors.New("禁止交付 Git 元数据")
		}
	}
	if change.OldMode != "000000" && change.Mode != "000000" && change.OldMode != change.Mode {
		return errors.New("Perforce 交付暂不支持修改现有文件的可执行权限")
	}
	for _, mode := range []string{change.Mode, change.OldMode} {
		if mode != "000000" && mode != "100644" && mode != "100755" {
			return errors.New("Perforce 交付不接受链接或特殊文件")
		}
	}
	if change.Size > 10*1024*1024 {
		return errors.New("Perforce 单文件交付暂限 10 MiB")
	}
	return nil
}

func exportP4File(ctx context.Context, root *os.Root, set *Fileset, file Change) error {
	if file.Status == "D" {
		err := root.Remove(filepath.FromSlash(file.Path))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data, err := set.Blob(ctx, file.Object)
	if err != nil {
		return err
	}
	name := filepath.FromSlash(file.Path)
	if err = root.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return err
	}
	if info, err := root.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return errors.New("交付目标被替换为链接或特殊文件")
	}
	mode := os.FileMode(0600)
	if file.Mode == "100755" {
		mode = 0700
	}
	if err = root.WriteFile(name, data, mode); err != nil {
		return err
	}
	return root.Chmod(name, mode)
}

func retainPerforce(ctx context.Context, conn *pgx.Conn, id, directory string) error {
	var review bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pangu.perforce_deliveries WHERE run_id=$1) OR EXISTS(SELECT 1 FROM pangu.perforce_sources WHERE run_id=$1 AND work_change>0)", id).Scan(&review); err != nil {
		return err
	}
	if review {
		return errors.New("Perforce 交付需要保留 Client 与待审核 shelf，不自动清理")
	}
	return nil
}

func removePerforceClient(ctx context.Context, conn *pgx.Conn, id, directory string) error {
	s, err := readPerforce(ctx, conn, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if s.LocalRoot != "" {
		return nil
	}
	return s.RemoveClient(ctx, filepath.Join(directory, "p4"))
}
