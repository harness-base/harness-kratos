-- Sandbox init: create greets table and seed one row for happy-path tests.

CREATE TABLE IF NOT EXISTS "greets" (
    "id"         BIGSERIAL PRIMARY KEY,
    "content"    VARCHAR NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL
);

-- Seed row so /v1/greet/1 returns data immediately.
INSERT INTO greets(id, content, created_at) VALUES (1, 'hello from sandbox', now());

-- Reset the sequence so auto-increment continues from 2.
SELECT setval('greets_id_seq', 1, true);
