package api

import (
	"context"
	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/Patrick08T/Pangu/internal/learning"
	"github.com/jackc/pgx/v5"
)

type currentBenefitReader interface {
	CurrentBenefitExperimentTx(context.Context, pgx.Tx, experience.Scope, string) (evaluation.ExperimentSpec, error)
}

func (h handler) deliveryStore() (experience.Store, bool) {
	service, ready := h.evolutionRuntime()
	reader, implemented := any(&service).(currentBenefitReader)
	if !ready || !implemented {
		return experience.Store{}, false
	}
	return learning.NewStoreWithBenefit(reader.CurrentBenefitExperimentTx), true
}
