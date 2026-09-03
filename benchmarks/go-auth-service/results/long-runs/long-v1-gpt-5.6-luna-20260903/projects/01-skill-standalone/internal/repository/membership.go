package repository

import (
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

type membershipKey struct {
	organizationID domain.OrganizationID
	userID         domain.UserID
}

// MembershipRepository is a concurrency-safe in-memory store. The composite
// index makes (organization,user) uniqueness atomic with insertion.
type MembershipRepository struct {
	mu                 sync.RWMutex
	memberships        map[domain.ID]domain.Membership
	byOrganizationUser map[membershipKey]domain.ID
}

func NewMembershipRepository() *MembershipRepository {
	return &MembershipRepository{
		memberships:        make(map[domain.ID]domain.Membership),
		byOrganizationUser: make(map[membershipKey]domain.ID),
	}
}

func (r *MembershipRepository) Create(membership domain.Membership) error {
	if membership.ID == "" || membership.OrganizationID == "" || membership.UserID == "" {
		return domain.NewInvalid("invalid membership")
	}
	key := membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.memberships[membership.ID]; exists {
		return domain.NewConflict("membership already exists")
	}
	if _, exists := r.byOrganizationUser[key]; exists {
		return domain.NewConflict("membership already exists")
	}
	r.memberships[membership.ID] = membership
	r.byOrganizationUser[key] = membership.ID
	return nil
}

func (r *MembershipRepository) GetByID(id domain.ID) (domain.Membership, error) {
	r.mu.RLock()
	membership, exists := r.memberships[id]
	r.mu.RUnlock()
	if !exists {
		return domain.Membership{}, domain.NewNotFound("membership not found")
	}
	return membership, nil
}

func (r *MembershipRepository) Get(organizationID domain.OrganizationID, userID domain.UserID) (domain.Membership, error) {
	key := membershipKey{organizationID: organizationID, userID: userID}
	r.mu.RLock()
	id, exists := r.byOrganizationUser[key]
	membership := r.memberships[id]
	r.mu.RUnlock()
	if !exists {
		return domain.Membership{}, domain.NewNotFound("membership not found")
	}
	return membership, nil
}

func (r *MembershipRepository) Update(membership domain.Membership) error {
	if membership.ID == "" || membership.OrganizationID == "" || membership.UserID == "" {
		return domain.NewInvalid("invalid membership")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	old, exists := r.memberships[membership.ID]
	if !exists {
		return domain.NewNotFound("membership not found")
	}
	key := membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID}
	if otherID, used := r.byOrganizationUser[key]; used && otherID != membership.ID {
		return domain.NewConflict("membership already exists")
	}
	oldKey := membershipKey{organizationID: old.OrganizationID, userID: old.UserID}
	if oldKey != key {
		delete(r.byOrganizationUser, oldKey)
		r.byOrganizationUser[key] = membership.ID
	}
	r.memberships[membership.ID] = membership
	return nil
}

// UpdateRole changes a member's built-in role while preserving the invariant
// that an organization always has an owner.
func (r *MembershipRepository) UpdateRole(organizationID domain.OrganizationID, userID domain.UserID, role string, now time.Time) (domain.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := membershipKey{organizationID: organizationID, userID: userID}
	id, exists := r.byOrganizationUser[key]
	if !exists {
		return domain.Membership{}, domain.NewNotFound("membership not found")
	}
	membership := r.memberships[id]
	if membership.Role == "owner" && role != "owner" && r.ownerCountLocked(organizationID) == 1 {
		return domain.Membership{}, domain.NewConflict("organization must have an owner")
	}
	membership.Role = role
	membership.UpdatedAt = now
	r.memberships[id] = membership
	return membership, nil
}

// RemoveRole replaces an assignment with the default viewer role. The
// assignment check and last-owner guard happen under the same lock so a
// concurrent role change cannot bypass the organization invariant.
func (r *MembershipRepository) RemoveRole(organizationID domain.OrganizationID, userID domain.UserID, role string, now time.Time) (domain.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := membershipKey{organizationID: organizationID, userID: userID}
	id, exists := r.byOrganizationUser[key]
	if !exists {
		return domain.Membership{}, domain.NewNotFound("membership not found")
	}
	membership := r.memberships[id]
	if membership.Role != role {
		return domain.Membership{}, domain.NewNotFound("role assignment not found")
	}
	if role == "owner" && r.ownerCountLocked(organizationID) == 1 {
		return domain.Membership{}, domain.NewConflict("organization must have an owner")
	}
	membership.Role = "viewer"
	membership.UpdatedAt = now
	r.memberships[id] = membership
	return membership, nil
}

// DeleteMember removes a member, refusing to remove an owner before transfer.
func (r *MembershipRepository) DeleteMember(organizationID domain.OrganizationID, userID domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := membershipKey{organizationID: organizationID, userID: userID}
	id, exists := r.byOrganizationUser[key]
	if !exists {
		return domain.NewNotFound("membership not found")
	}
	if r.memberships[id].Role == "owner" {
		return domain.NewConflict("ownership must be transferred first")
	}
	delete(r.memberships, id)
	delete(r.byOrganizationUser, key)
	return nil
}

// TransferOwnership changes both membership roles under one repository lock.
// The caller updates the organization's denormalized OwnerID while holding
// the same application-level organization lock.
func (r *MembershipRepository) TransferOwnership(organizationID domain.OrganizationID, currentOwnerID, newOwnerID domain.UserID, now time.Time) (domain.Membership, domain.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if currentOwnerID == newOwnerID {
		return domain.Membership{}, domain.Membership{}, domain.NewConflict("new owner must be different")
	}
	oldID, exists := r.byOrganizationUser[membershipKey{organizationID: organizationID, userID: currentOwnerID}]
	if !exists || r.memberships[oldID].Role != "owner" {
		return domain.Membership{}, domain.Membership{}, domain.NewConflict("current owner required")
	}
	newID, exists := r.byOrganizationUser[membershipKey{organizationID: organizationID, userID: newOwnerID}]
	if !exists {
		return domain.Membership{}, domain.Membership{}, domain.NewNotFound("membership not found")
	}
	old := r.memberships[oldID]
	newOwner := r.memberships[newID]
	old.Role = "admin"
	old.UpdatedAt = now
	newOwner.Role = "owner"
	newOwner.UpdatedAt = now
	r.memberships[oldID] = old
	r.memberships[newID] = newOwner
	return old, newOwner, nil
}

func (r *MembershipRepository) ownerCountLocked(organizationID domain.OrganizationID) int {
	count := 0
	for _, membership := range r.memberships {
		if membership.OrganizationID == organizationID && membership.Role == "owner" {
			count++
		}
	}
	return count
}

func (r *MembershipRepository) Delete(id domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	membership, exists := r.memberships[id]
	if !exists {
		return domain.NewNotFound("membership not found")
	}
	delete(r.memberships, id)
	delete(r.byOrganizationUser, membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID})
	return nil
}

func (r *MembershipRepository) ListByOrganization(organizationID domain.OrganizationID) []domain.Membership {
	r.mu.RLock()
	result := make([]domain.Membership, 0)
	for _, membership := range r.memberships {
		if membership.OrganizationID == organizationID {
			result = append(result, membership)
		}
	}
	r.mu.RUnlock()
	return result
}

func (r *MembershipRepository) ListByUser(userID domain.UserID) []domain.Membership {
	r.mu.RLock()
	result := make([]domain.Membership, 0)
	for _, membership := range r.memberships {
		if membership.UserID == userID {
			result = append(result, membership)
		}
	}
	r.mu.RUnlock()
	return result
}
