-- Initial schema for demo service.
-- Hand-written to match the ent schema (app/demo/internal/data/ent/schema/greet.go).
-- Atlas generation (atlas migrate diff initial --dev-url "docker://postgres/16-alpine/dev")
-- is blocked by the Go `internal` package restriction in the atlas ent loader, so this
-- migration is maintained by hand and applied to the sandbox via initdb
-- (deploy/sandbox/initdb). To switch to atlas generation, move the schema outside
-- `internal` or add an entc config shim.

CREATE TABLE "greets" (
    "id"         BIGSERIAL NOT NULL,
    "content"    CHARACTER VARYING NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL,
    PRIMARY KEY ("id")
);
