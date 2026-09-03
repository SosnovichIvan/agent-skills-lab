package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

type membershipKey struct {
	organizationID domain.OrganizationID
	userID         domain.UserID
}

type MembershipRepository struct {
	mu   sync.RWMutex
	byID map[membershipKey]domain.Membership
}

func NewMembershipRepository() *MembershipRepository {
	return &MembershipRepository{byID: make(map[membershipKey]domain.Membership)}
}

func (r *MembershipRepository) Create(membership domain.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if membership.OrganizationID == "" || membership.UserID == "" || membership.Role == "" {
		return domain.Invalid("membership is incomplete")
	}
	key := membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID}
	if _, exists := r.byID[key]; exists {
		return domain.Conflict("membership already exists")
	}
	r.byID[key] = membership
	return nil
}

func (r *MembershipRepository) Get(organizationID domain.OrganizationID, userID domain.UserID) (domain.Membership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	membership, exists := r.byID[membershipKey{organizationID: organizationID, userID: userID}]
	if !exists {
		return domain.Membership{}, domain.NotFound("membership not found")
	}
	return membership, nil
}

func (r *MembershipRepository) ListByOrganization(organizationID domain.OrganizationID) []domain.Membership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	memberships := make([]domain.Membership, 0)
	for key, membership := range r.byID {
		if key.organizationID == organizationID {
			memberships = append(memberships, membership)
		}
	}
	return memberships
}

func (r *MembershipRepository) ListByUser(userID domain.UserID) []domain.Membership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	memberships := make([]domain.Membership, 0)
	for _, membership := range r.byID {
		if membership.UserID == userID {
			memberships = append(memberships, membership)
		}
	}
	return memberships
}

func (r *MembershipRepository) Update(membership domain.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID}
	if _, exists := r.byID[key]; !exists {
		return domain.NotFound("membership not found")
	}
	if membership.Role == "" {
		return domain.Invalid("membership role is required")
	}
	r.byID[key] = membership
	return nil
}

func (r *MembershipRepository) Delete(organizationID domain.OrganizationID, userID domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := membershipKey{organizationID: organizationID, userID: userID}
	if _, exists := r.byID[key]; !exists {
		return domain.NotFound("membership not found")
	}
	delete(r.byID, key)
	return nil
}

func (r *MembershipRepository) CountRole(organizationID domain.OrganizationID, role string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for key, membership := range r.byID {
		if key.organizationID == organizationID && membership.Role == role {
			count++
		}
	}
	return count
}

// TransferOwnership changes both memberships under one repository lock.
func (r *MembershipRepository) TransferOwnership(organizationID domain.OrganizationID, currentOwner, newOwner domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	currentKey := membershipKey{organizationID: organizationID, userID: currentOwner}
	newKey := membershipKey{organizationID: organizationID, userID: newOwner}
	current, exists := r.byID[currentKey]
	if !exists || current.Role != "owner" {
		return domain.Unauthorized("current user is not the owner")
	}
	newMembership, exists := r.byID[newKey]
	if !exists {
		return domain.NotFound("new owner membership not found")
	}
	if currentOwner == newOwner {
		return domain.Conflict("new owner is already the owner")
	}
	now := current.UpdatedAt
	current.Role = "admin"
	current.UpdatedAt = now
	newMembership.Role = "owner"
	newMembership.UpdatedAt = now
	r.byID[currentKey] = current
	r.byID[newKey] = newMembership
	return nil
}
