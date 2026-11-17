package perforce

import (
	"context"
	"errors"
)

// recordStream frames complete JSON objects, independent of lines and Write boundaries.
// The bounded frame is parsed by the same parser as ordinary commands.
type recordStream struct {
	recordLimit, totalLimit int
	total, depth            int
	quoted, escaped         bool
	frame                   []byte
	err                     error
	cancel                  context.CancelFunc
	consume                 func(Record) error
}

func (s *recordStream) fail(err error) error {
	if s.err == nil {
		s.err = err
		if s.cancel != nil {
			s.cancel()
		}
	}
	return s.err
}

func (s *recordStream) Write(p []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	if len(p) > s.totalLimit-s.total {
		return 0, s.fail(errors.New("Perforce 文件清单输出超过 512MiB"))
	}
	s.total += len(p)
	for i, b := range p {
		if len(s.frame) == 0 && (b == ' ' || b == '\r' || b == '\n' || b == '\t') {
			continue
		}
		if len(s.frame) == 0 && b != '{' {
			return i, s.fail(errors.New("Perforce 返回无效 JSON"))
		}
		if len(s.frame) >= s.recordLimit {
			return i, s.fail(errors.New("Perforce 文件清单单条记录超过 1MiB"))
		}
		s.frame = append(s.frame, b)
		if s.quoted {
			if s.escaped {
				s.escaped = false
			} else if b == '\\' {
				s.escaped = true
			} else if b == '"' {
				s.quoted = false
			}
		} else {
			switch b {
			case '"':
				s.quoted = true
			case '{', '[':
				s.depth++
			case '}', ']':
				s.depth--
			}
		}
		if s.depth == 0 {
			rows, err := parse(string(s.frame))
			if err != nil {
				return i + 1, s.fail(err)
			}
			if len(rows) != 1 {
				return i + 1, s.fail(errors.New("Perforce 返回无效 JSON"))
			}
			if err = s.consume(rows[0]); err != nil {
				return i + 1, s.fail(err)
			}
			s.frame = s.frame[:0]
		}
	}
	return len(p), nil
}

func (s *recordStream) finish() error {
	if s.err != nil {
		return s.err
	}
	if len(s.frame) != 0 {
		return s.fail(errors.New("Perforce 返回无效 JSON"))
	}
	return nil
}
