package learning

import (
	"errors"

	"github.com/Patrick08T/Pangu/internal/evaluation"
	"github.com/Patrick08T/Pangu/internal/experience"
	"github.com/jackc/pgx/v5"
)

func memoryGuardFailure(err error) bool {
	for _, target := range []error{experience.ErrInvalid, experience.ErrForbidden, experience.ErrNotFound, experience.ErrConflict, experience.ErrInvalidated, evaluation.ErrInvalid, evaluation.ErrNotFound, evaluation.ErrExpired, evaluation.ErrConflict, pgx.ErrNoRows} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
