package evolution

import (
	"context"
	"errors"
	"testing"

	"github.com/Patrick08T/Pangu/internal/experience"
)

func TestBenefitGuardClassifiesDomainFailuresButPreservesStorageErrors(t *testing.T) {
	for _, err := range []error{ErrForbidden, ErrConflict, ErrNotFound, experience.ErrConflict} {
		if !errors.Is(benefitGuardError(err), experience.ErrInvalidated) {
			t.Fatal("依赖变化未分类invalidated", err)
		}
	}
	storage := errors.New("connection unavailable")
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, storage} {
		if benefitGuardError(err) != err {
			t.Fatal("基础设施故障被当成业务暂停", err)
		}
	}
	if benefitGuardError(nil) != nil {
		t.Fatal("成功被改为失败")
	}
}
