#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

for f in store/migrations/*.sql; do
  version=$(basename "$f")
  applied=0
  if check=$(docker compose exec -T postgres psql -U atlas -d atlas -tAc \
    "SELECT 1 FROM schema_migrations WHERE version='${version}'" 2>/dev/null); then
    if echo "$check" | grep -q 1; then
      applied=1
    fi
  fi

  if [ "$applied" -eq 1 ]; then
    echo "Skipping $f (already applied)"
    continue
  fi

  echo "Applying $f..."
  docker compose exec -T postgres psql -U atlas -d atlas -v ON_ERROR_STOP=1 < "$f"
  docker compose exec -T postgres psql -U atlas -d atlas \
    -c "INSERT INTO schema_migrations (version) VALUES ('${version}')"
done

echo "Migrations complete."
