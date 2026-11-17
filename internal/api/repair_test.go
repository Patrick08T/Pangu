package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/approval"
)

func (b *fakeBackend) PrepareRepair(context.Context, string, approval.RepairInput) (*approval.Repair, error) {
	return nil, approval.ErrInvalid
}
