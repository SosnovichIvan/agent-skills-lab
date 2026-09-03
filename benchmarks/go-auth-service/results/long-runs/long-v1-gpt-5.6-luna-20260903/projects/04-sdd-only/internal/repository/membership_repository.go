package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

type membershipKey struct {
	organizationID domain.ID
	userID         domain.ID
}

type MembershipStore interface {
	Create(membership domain.Membership) error
	Get(organizationID, userID domain.ID) (domain.Membership, error)
	ListByOrganization(organizationID domain.ID) []domain.Membership
	ListByUser(userID domain.ID) []domain.Membership
	Update(membership domain.Membership) error
	Delete(organizationID, userID domain.ID) error
	TransferOwnership(organizationID, currentOwnerID, newOwnerID domain.ID) error
}

type MembershipRepository struct {
	mu    sync.RWMutex
	byKey map[membershipKey]domain.Membership
}

func NewMembershipRepository() *MembershipRepository {
	return &MembershipRepository{byKey: make(map[membershipKey]domain.Membership)}
}

func (r *MembershipRepository) Create(membership domain.Membership) error {
	if err := validateMembership(membership); err != nil {
		return err
	}
	key := membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byKey[key]; exists {
		return domain.NewError(domain.KindConflict, "membership already exists")
	}
	r.byKey[key] = cloneMembership(membership)
	return nil
}

func (r *MembershipRepository) Get(organizationID, userID domain.ID) (domain.Membership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	membership, ok := r.byKey[membershipKey{organizationID: organizationID, userID: userID}]
	if !ok {
		return domain.Membership{}, domain.NewError(domain.KindNotFound, "membership not found")
	}
	return cloneMembership(membership), nil
}

func (r *MembershipRepository) ListByOrganization(organizationID domain.ID) []domain.Membership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.Membership, 0)
	for _, membership := range r.byKey {
		if membership.OrganizationID == organizationID {
			result = append(result, cloneMembership(membership))
		}
	}
	return result
}

func (r *MembershipRepository) ListByUser(userID domain.ID) []domain.Membership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.Membership, 0)
	for _, membership := range r.byKey {
		if membership.UserID == userID {
			result = append(result, cloneMembership(membership))
		}
	}
	return result
}

func (r *MembershipRepository) Update(membership domain.Membership) error {
	if err := validateMembership(membership); err != nil {
		return err
	}
	key := membershipKey{organizationID: membership.OrganizationID, userID: membership.UserID}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byKey[key]; !exists {
		return domain.NewError(domain.KindNotFound, "membership not found")
	}
	current := r.byKey[key]
	if current.Role == domain.OwnerRole && membership.Role != domain.OwnerRole && r.ownerCountLocked(membership.OrganizationID) <= 1 {
		return domain.NewError(domain.KindConflict, "last organization owner cannot lose owner role")
	}
	r.byKey[key] = cloneMembership(membership)
	return nil
}

func (r *MembershipRepository) Delete(organizationID, userID domain.ID) error {
	key := membershipKey{organizationID: organizationID, userID: userID}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byKey[key]; !exists {
		return domain.NewError(domain.KindNotFound, "membership not found")
	}
	if r.byKey[key].Role == domain.OwnerRole && r.ownerCountLocked(organizationID) <= 1 {
		return domain.NewError(domain.KindConflict, "last organization owner cannot be removed")
	}
	delete(r.byKey, key)
	return nil
}

// TransferOwnership atomically changes both membership roles and protects
// the organization from ever having zero owners.
func (r *MembershipRepository) TransferOwnership(organizationID, currentOwnerID, newOwnerID domain.ID) error {
	if organizationID == "" || currentOwnerID == "" || newOwnerID == "" || currentOwnerID == newOwnerID {
		return domain.NewError(domain.KindInvalid, "ownership transfer is invalid")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	currentKey := membershipKey{organizationID: organizationID, userID: currentOwnerID}
	newKey := membershipKey{organizationID: organizationID, userID: newOwnerID}
	current, currentExists := r.byKey[currentKey]
	newMember, newExists := r.byKey[newKey]
	if !currentExists || !newExists {
		return domain.NewError(domain.KindNotFound, "membership not found")
	}
	if current.Role != domain.OwnerRole {
		return domain.NewError(domain.KindUnauthorized, "current user is not owner")
	}
	current.Role = domain.AdminRole
	newMember.Role = domain.OwnerRole
	r.byKey[currentKey] = cloneMembership(current)
	r.byKey[newKey] = cloneMembership(newMember)
	return nil
}

func (r *MembershipRepository) ownerCountLocked(organizationID domain.ID) int {
	count := 0
	for _, membership := range r.byKey {
		if membership.OrganizationID == organizationID && membership.Role == domain.OwnerRole {
			count++
		}
	}
	return count
}

func validateMembership(membership domain.Membership) error {
	if membership.OrganizationID == "" || membership.UserID == "" {
		return domain.NewError(domain.KindInvalid, "membership organization and user are required")
	}
	if membership.Role == "" {
		return domain.NewError(domain.KindInvalid, "membership role is required")
	}
	return nil
}

func cloneMembership(membership domain.Membership) domain.Membership {
	membership.RoleIDs = append([]domain.ID(nil), membership.RoleIDs...)
	return membership
}
