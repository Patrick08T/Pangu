package workspace

import (
	"context"
	"errors"
)

func (set *Fileset) documentSize(ctx context.Context, object string) error {
	size, err := set.size(ctx, object)
	if err != nil || size > 64*1024 {
		unavailable := errors.New("资料不可读或超过 64 KiB")
		if err != nil && set.localPerforce {
			return errors.Join(err, unavailable)
		}
		return unavailable
	}
	return nil
}
