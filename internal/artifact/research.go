package artifact

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"github.com/Patrick08T/Pangu/internal/research"
	"github.com/Patrick08T/Pangu/internal/workspace"
)

func reportFile(record research.Record) File {
	hash := sha256.Sum256(record.Body)
	file := File{ID: record.ID, Path: ".pangu/reports/" + record.ID + ".md", Change: "A", Size: int64(len(record.Body)), SHA256: hex.EncodeToString(hash[:])}
	if diagnostic.Sensitive(record.Body) {
		file.Blocked = "sensitive"
	}
	return file
}

func readReport(ctx context.Context, run, id string) (File, []byte, bool, error) {
	records, err := research.Reports(ctx, run)
	if err != nil {
		return File{}, nil, false, err
	}
	for _, record := range records {
		if record.ID != id {
			continue
		}
		file := reportFile(record)
		if file.Blocked != "" {
			return file, nil, false, ErrBlocked
		}
		return file, record.Body, false, nil
	}
	return File{}, nil, false, ErrNotFound
}

func exportReports(archive *zip.Writer, records []research.Record, manifest *Manifest) error {
	for _, record := range records {
		file := reportFile(record)
		if file.Blocked != "" {
			return ErrBlocked
		}
		entry, err := archive.Create(file.Path)
		if err != nil {
			return err
		}
		if _, err = entry.Write(record.Body); err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, file)
	}
	return nil
}

func artifactBudget(set *workspace.Fileset, reports []research.Record) (int64, error) {
	if len(set.Changes)+len(reports) > 1000 {
		return 0, errors.New("产物超过 1000 个文件")
	}
	var total, inspection int64
	for _, change := range set.Changes {
		total += change.Size
		inspection += change.Size + change.OldSize
	}
	for _, report := range reports {
		total += int64(len(report.Body))
		inspection += int64(len(report.Body))
	}
	if total > 100*1024*1024 || inspection > 100*1024*1024 {
		return 0, errors.New("产物导出或新旧内容安全扫描超过 100 MiB 总上限")
	}
	return total, nil
}
