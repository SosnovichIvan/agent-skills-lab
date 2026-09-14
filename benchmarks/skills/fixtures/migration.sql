BEGIN;
CREATE INDEX CONCURRENTLY orders_created_at_idx ON orders(created_at);
COMMIT;
