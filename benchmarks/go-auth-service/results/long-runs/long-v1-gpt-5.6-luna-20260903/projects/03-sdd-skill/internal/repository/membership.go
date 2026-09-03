package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

type membershipKey struct {
	organizationID domain.OrganizationID
	userID         domain.UserID
}

// MembershipRepository is a concurrency-safe in-memory membership store. The
// pair index and primary store are changed under one lock, making the
// organization/user uniqueness guarantee atomic under concurrent creates.
type MembershipRepository struct {
	mu          sync.RWMutex
	memberships map[domain.MembershipID]domain.Membership
	pairIndex   map[membershipKey]domain.MembershipID
}

func NewMembershipRepository() *MembershipRepository {
	return &MembershipRepository{
		memberships: make(map[domain.MembershipID]domain.Membership),
		pairIndex:   make(map[membershipKey]domain.MembershipID),
	}
}

func (r *MembershipRepository) Create(membership domain.Membership) error {
	if r == nil || membership.ID == "" || membership.OrganizationID == "" || membership.UserID == "" {
		return domain.NewError(domain.ErrorInvalid, "invalid membership")
	}
	key := membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.memberships[membership.ID]; exists {
		return domain.NewError(domain.ErrorConflict, "membership already exists")
	}
	if _, exists := r.pairIndex[key]; exists {
		return domain.NewError(domain.ErrorConflict, "membership already exists")
	}
	r.memberships[membership.ID] = cloneMembership(membership)
	r.pairIndex[key] = membership.ID
	return nil
}

func (r *MembershipRepository) GetByID(id domain.MembershipID) (domain.Membership, error) {
	if r == nil {
		return domain.Membership{}, domain.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	membership, ok := r.memberships[id]
	if !ok {
		return domain.Membership{}, domain.ErrNotFound
	}
	return cloneMembership(membership), nil
}

func (r *MembershipRepository) GetByOrganizationAndUser(organizationID domain.OrganizationID, userID domain.UserID) (domain.Membership, error) {
	if r == nil {
		return domain.Membership{}, domain.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.pairIndex[membershipKey{organizationID: organizationID, userID: userID}]
	if !ok {
		return domain.Membership{}, domain.ErrNotFound
	}
	return cloneMembership(r.memberships[id]), nil
}

func (r *MembershipRepository) ListByOrganization(organizationID domain.OrganizationID) []domain.Membership {
	return r.list(func(membership domain.Membership) bool { return membership.OrganizationID == organizationID })
}

func (r *MembershipRepository) ListByUser(userID domain.UserID) []domain.Membership {
	return r.list(func(membership domain.Membership) bool { return membership.UserID == userID })
}

func (r *MembershipRepository) list(matches func(domain.Membership) bool) []domain.Membership {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.Membership, 0)
	for _, membership := range r.memberships {
		if matches(membership) {
			result = append(result, cloneMembership(membership))
		}
	}
	return result
}

func (r *MembershipRepository) Update(membership domain.Membership) error {
	if r == nil || membership.ID == "" || membership.OrganizationID == "" || membership.UserID == "" {
		return domain.NewError(domain.ErrorInvalid, "invalid membership")
	}
	key := membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID}
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, exists := r.memberships[membership.ID]
	if !exists {
		return domain.ErrNotFound
	}
	if owner, claimed := r.pairIndex[key]; claimed && owner != membership.ID {
		return domain.NewError(domain.ErrorConflict, "membership already exists")
	}
	oldKey := membershipKey{organizationID: previous.OrganizationID, userID: previous.UserID}
	if oldKey != key {
		delete(r.pairIndex, oldKey)
	}
	r.memberships[membership.ID] = cloneMembership(membership)
	r.pairIndex[key] = membership.ID
	return nil
}

func cloneMembership(membership domain.Membership) domain.Membership {
	membership.RoleIDs = append([]domain.RoleID(nil), membership.RoleIDs...)
	return membership
}

func (r *MembershipRepository) Delete(id domain.MembershipID) error {
	if r == nil {
		return domain.ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	membership, exists := r.memberships[id]
	if !exists {
		return domain.ErrNotFound
	}
	delete(r.memberships, id)
	delete(r.pairIndex, membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID})
	return nil
}
