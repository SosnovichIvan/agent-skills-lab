package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

type roleAssignmentKey struct {
	organizationID domain.OrganizationID
	userID         domain.UserID
	roleID         domain.RoleID
}

type RoleAssignmentRepository struct {
	mu    sync.RWMutex
	byKey map[roleAssignmentKey]domain.RoleAssignment
}

func NewRoleAssignmentRepository() *RoleAssignmentRepository {
	return &RoleAssignmentRepository{byKey: make(map[roleAssignmentKey]domain.RoleAssignment)}
}

func (r *RoleAssignmentRepository) Create(assignment domain.RoleAssignment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if assignment.OrganizationID == "" || assignment.UserID == "" || assignment.RoleID == "" {
		return domain.Invalid("role assignment is incomplete")
	}
	key := roleAssignmentKey{assignment.OrganizationID, assignment.UserID, assignment.RoleID}
	if _, exists := r.byKey[key]; exists {
		return domain.Conflict("role is already assigned")
	}
	r.byKey[key] = assignment
	return nil
}

func (r *RoleAssignmentRepository) Delete(organizationID domain.OrganizationID, userID domain.UserID, roleID domain.RoleID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := roleAssignmentKey{organizationID, userID, roleID}
	if _, exists := r.byKey[key]; !exists {
		return domain.NotFound("role assignment not found")
	}
	delete(r.byKey, key)
	return nil
}

func (r *RoleAssignmentRepository) ListByMember(organizationID domain.OrganizationID, userID domain.UserID) []domain.RoleAssignment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	assignments := make([]domain.RoleAssignment, 0)
	for key, assignment := range r.byKey {
		if key.organizationID == organizationID && key.userID == userID {
			assignments = append(assignments, assignment)
		}
	}
	return assignments
}

func (r *RoleAssignmentRepository) Count(organizationID domain.OrganizationID, roleID domain.RoleID) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for key := range r.byKey {
		if key.organizationID == organizationID && key.roleID == roleID {
			count++
		}
	}
	return count
}
