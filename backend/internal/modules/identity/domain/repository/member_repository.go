package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MemberRepository interface {
	Save(ctx context.Context, member *model.Member) error
	FindByEmail(ctx context.Context, email string) (*model.Member, error)
	FindByID(ctx context.Context, id kernel.MemberID) (*model.Member, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	ListAll(ctx context.Context) ([]*model.Member, error)
	SoftDelete(ctx context.Context, id kernel.MemberID) error
}
