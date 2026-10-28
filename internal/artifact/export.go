package artifact

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/Patrick08T/Pangu/internal/research"

	"github.com/Patrick08T/Pangu/internal/workspace"
)

func Export(ctx context.Context, run string) ([]byte, error) {
	release, err := workspace.Lease(ctx, run)
	if err != nil {
		return nil, err
	}
	defer release()
	return export(ctx, run)
}

func export(ctx context.Context, run string) ([]byte, error) {
	set, err := workspace.Files(ctx, run)
	if err != nil {
		return nil, err
	}
	reports, err := research.Reports(ctx, run)
	if err != nil {
		return nil, err
	}
	total, err := artifactBudget(set, reports)
	if err != nil {
		return nil, err
	}
	buffer := &bytes.Buffer{}
	archive := zip.NewWriter(buffer)
	defer archive.Close()
	manifest := Manifest{RunID: run, BaseCommit: set.Base, HeadCommit: set.Head, Files: []File{}, TotalBytes: total}
	for _, change := range set.Changes {
		file, data, err := inspect(ctx, set, change)
		if err != nil {
			return nil, err
		}
		if file.Blocked != "" {
			return nil, ErrBlocked
		}
		manifest.Files = append(manifest.Files, file)
		if file.Change == "D" {
			continue
		}
		entry, err := archive.Create(file.Path)
		if err != nil {
			return nil, err
		}
		if _, err = entry.Write(data); err != nil {
			return nil, err
		}
	}
	if err = exportReports(archive, reports, &manifest); err != nil {
		return nil, err
	}
	entry, err := archive.Create(".pangu/manifest.json")
	if err != nil {
		return nil, err
	}
	if err = json.NewEncoder(entry).Encode(manifest); err != nil {
		return nil, err
	}
	if err = archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
