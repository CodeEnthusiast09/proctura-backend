# Proctura Backend — Makefile
# Usage: make <target>

.PHONY: run build test migrate-diff migrate-hash migrate-apply migrate-status migrate-validate clean

# ── App ───────────────────────────────────────────────────────────────────────

run:
	go run ./cmd/main.go

build:
	go build -o ./bin/proctura ./cmd/main.go

clean:
	rm -rf ./bin

# ── Tests ─────────────────────────────────────────────────────────────────────

test:
	go test ./... -v -p 1

# ── Migrations ────────────────────────────────────────────────────────────────

# Generate a new migration from model changes.
# Usage: make migrate-diff name=add_phone_number_to_users
migrate-diff:
	@[ -n "$(name)" ] || (echo "Error: provide a name — make migrate-diff name=<descriptive_name>" && exit 1)
	atlas migrate diff $(name) --env gorm

# Rehash the migration directory after adding or editing any .sql file.
# Always run this after migrate-diff before committing.
migrate-hash:
	atlas migrate hash --env gorm

# Apply pending migrations to the database manually.
# The app also does this automatically on startup.
# Usage: make migrate-apply url=postgres://user:pass@localhost:5432/proctura_db
migrate-apply:
	@[ -n "$(url)" ] || (echo "Error: provide a url — make migrate-apply url=<DSN>" && exit 1)
	atlas migrate apply --env gorm --url "$(url)"

# Show which migrations are pending vs applied.
# Usage: make migrate-status url=postgres://user:pass@localhost:5432/proctura_db
migrate-status:
	@[ -n "$(url)" ] || (echo "Error: provide a url — make migrate-status url=<DSN>" && exit 1)
	atlas migrate status --env gorm --url "$(url)"

# Validate the migration directory integrity against atlas.sum.
migrate-validate:
	atlas migrate validate --env gorm
