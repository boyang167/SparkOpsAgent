package devmemory

import (
	"context"
	"sync"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MemberRepository struct {
	mu      sync.Mutex
	byEmail map[string]*model.Member
	byID    map[string]*model.Member
}

func NewMemberRepository() *MemberRepository {
	return &MemberRepository{
		byEmail: make(map[string]*model.Member),
		byID:    make(map[string]*model.Member),
	}
}

func (r *MemberRepository) Save(_ context.Context, member *model.Member) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyMember := *member
	r.byEmail[member.Email] = &copyMember
	r.byID[member.ID.String()] = &copyMember
	return nil
}

func (r *MemberRepository) FindByEmail(_ context.Context, email string) (*model.Member, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byEmail[email]
	if !ok {
		return nil, domain.ErrMemberNotFound
	}
	copyMember := *m
	return &copyMember, nil
}

func (r *MemberRepository) FindByID(_ context.Context, id kernel.MemberID) (*model.Member, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byID[id.String()]
	if !ok {
		return nil, domain.ErrMemberNotFound
	}
	copyMember := *m
	return &copyMember, nil
}

func (r *MemberRepository) ExistsByEmail(_ context.Context, email string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.byEmail[email]
	return ok, nil
}

var _ repository.MemberRepository = (*MemberRepository)(nil)
