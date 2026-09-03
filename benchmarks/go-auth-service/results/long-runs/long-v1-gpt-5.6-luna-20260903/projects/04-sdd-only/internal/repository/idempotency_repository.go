package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

type IdempotencyRecord struct {
	Fingerprint []byte
	Status      int
	Body        []byte
}

type IdempotencyStore interface {
	Get(key string) (IdempotencyRecord, bool)
	Put(key string, record IdempotencyRecord) error
}

type IdempotencyRepository struct {
	mu      sync.RWMutex
	records map[string]IdempotencyRecord
}

func NewIdempotencyRepository() *IdempotencyRepository {
	return &IdempotencyRepository{records: make(map[string]IdempotencyRecord)}
}

func (r *IdempotencyRepository) Get(key string) (IdempotencyRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.records[key]
	if !ok {
		return IdempotencyRecord{}, false
	}
	return cloneIdempotencyRecord(record), true
}

func (r *IdempotencyRepository) Put(key string, record IdempotencyRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if previous, exists := r.records[key]; exists {
		if string(previous.Fingerprint) != string(record.Fingerprint) {
			return domain.NewError(domain.KindConflict, "idempotency key reused with different request")
		}
		return nil
	}
	r.records[key] = cloneIdempotencyRecord(record)
	return nil
}

func cloneIdempotencyRecord(record IdempotencyRecord) IdempotencyRecord {
	record.Fingerprint = append([]byte(nil), record.Fingerprint...)
	record.Body = append([]byte(nil), record.Body...)
	return record
}
