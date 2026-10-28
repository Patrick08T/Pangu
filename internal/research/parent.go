package research

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
)

func (s *Session) readParent(ctx context.Context, parentID string, page int) (string, error) {
	parent, ok := s.input.Parents[parentID]
	if !ok || len(parent.ReportIDs) != 1 || parent.RunID == "" {
		return "", errors.New("没有该直接上游的有效报告")
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	var body []byte
	err = conn.QueryRow(ctx, `SELECT body FROM pangu.research_reports WHERE run_id=$1 AND node_run_id=$2 AND id=$3`, s.input.RunID, parent.RunID, parent.ReportIDs[0]).Scan(&body)
	if err != nil {
		return "", errors.New("直接上游报告不存在或已过期")
	}
	if diagnostic.Sensitive(body) {
		return "", errors.New("上游报告疑似包含凭据")
	}
	runes := []rune(string(body))
	pages := (len(runes) + 16383) / 16384
	if page >= pages {
		return "", errors.New("报告页码超出范围")
	}
	text := string(runes[page*16384 : min((page+1)*16384, len(runes))])
	hash := sha256.Sum256([]byte(text))
	id := fmt.Sprintf("p%s_%d", parentID, page)
	source := Source{ID: id, Title: fmt.Sprintf("上游 %s 报告，第 %d/%d 页（其事实仍需核对原来源）", parentID, page+1, pages), Read: true, RetrievedAt: time.Now().UTC(), SHA256: hex.EncodeToString(hash[:]), ArtifactID: parent.ReportIDs[0]}
	s.sources[id] = source
	return sourceResult(source, text)
}
