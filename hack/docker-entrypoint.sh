#!/bin/sh
set -eu

MIGRATIONS_DIR="${ATLAS_MIGRATIONS_DIR:-/migrations}"

wait_for_db() {
	if [ -z "${ATLAS_DATABASE_URL:-}" ]; then
		echo "ATLAS_DATABASE_URL is required" >&2
		exit 1
	fi
	echo "Waiting for Postgres..."
	i=0
	while [ "$i" -lt 60 ]; do
		if psql "$ATLAS_DATABASE_URL" -v ON_ERROR_STOP=1 -c "SELECT 1" >/dev/null 2>&1; then
			echo "Postgres is ready."
			return 0
		fi
		i=$((i + 1))
		sleep 1
	done
	echo "Postgres did not become ready in time" >&2
	exit 1
}

apply_migrations() {
	echo "Applying migrations from ${MIGRATIONS_DIR}..."
	for f in "$MIGRATIONS_DIR"/*.sql; do
		[ -f "$f" ] || continue
		version=$(basename "$f")
		applied=$(psql "$ATLAS_DATABASE_URL" -tAc \
			"SELECT 1 FROM schema_migrations WHERE version='${version}'" 2>/dev/null || true)
		applied=$(echo "$applied" | tr -d '[:space:]')
		if [ "$applied" = "1" ]; then
			echo "Skipping $version (already applied)"
			continue
		fi
		echo "Applying $version..."
		psql "$ATLAS_DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f"
		# 000 creates schema_migrations; track it (and all others) after apply
		psql "$ATLAS_DATABASE_URL" -v ON_ERROR_STOP=1 -c \
			"INSERT INTO schema_migrations (version) VALUES ('${version}') ON CONFLICT DO NOTHING"
	done
	echo "Migrations complete."
}

wait_for_db
apply_migrations

exec "$@"
