# Shared helpers for the scripts/import-lab-*.sh scripts. Source it, don't run it.
#
# Needs LAB_PASSWORD. Optional: LAB_HOST (default host.docker.internal, i.e. the
# Docker host), LAB_PORT (5432), LAB_DB (lab), LAB_USER (postgres).
# Rows go to DATABASE_URL (env or .env) when set, else the "db" container.
# Both databases are reached through psql in the "db" container, and rows move
# with \copy, so it works across PostgreSQL versions.

: "${LAB_PASSWORD:?Set LAB_PASSWORD to the password of the lab database}"
LAB_HOST="${LAB_HOST:-host.docker.internal}"
LAB_PORT="${LAB_PORT:-5432}"
LAB_DB="${LAB_DB:-lab}"
LAB_USER="${LAB_USER:-postgres}"
export MSYS_NO_PATHCONV=1 # Git Bash on Windows: do not rewrite arguments

# psql against the lab database
lab() { docker compose exec -T -e PGPASSWORD="$LAB_PASSWORD" db psql -X -q -v ON_ERROR_STOP=1 -h "$LAB_HOST" -p "$LAB_PORT" -U "$LAB_USER" -d "$LAB_DB" "$@"; }
# The app database: the backend's DATABASE_URL (env or .env) when set, so imports
# land where the backend reads; otherwise the "db" container's own database.
if [[ -z "${DATABASE_URL:-}" && -f .env ]]; then
  DATABASE_URL=$(sed -n 's/^DATABASE_URL=//p' .env | tail -n 1)
fi
# psql against the app database
if [[ -n "${DATABASE_URL:-}" ]]; then
  app() { docker compose exec -T db psql -X -q -v ON_ERROR_STOP=1 "$DATABASE_URL" "$@"; }
else
  app() { docker compose exec -T db sh -c 'psql -X -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" "$@"' psql "$@"; }
fi

# require_empty TABLE REPLACE: stop unless TABLE is empty or REPLACE is 1.
require_empty() {
  local n
  n=$(app -At -c "SELECT count(*) FROM $1")
  if [[ "$n" != "0" && "$2" != "1" ]]; then
    echo "$1 already has $n rows. Re-run with --replace to wipe and re-import." >&2
    exit 1
  fi
}

# copy_query TABLE "COLUMNS" "SELECT ...": load the lab query's rows into TABLE (COLUMNS).
copy_query() {
  lab -c "\\copy ($3) TO STDOUT" | app -c "\\copy $1 ($2) FROM STDIN"
  printf '  %-20s %s rows\n' "$1" "$(app -At -c "SELECT count(*) FROM $1")"
}

# copy TABLE "COLUMNS" [WHERE]: same column list on both sides.
copy() { copy_query "$1" "$2" "SELECT $2 FROM $1 ${3:-}"; }

# app_branches: the app's branch codes as a quoted SQL list, e.g. 'CTM','MED'.
app_branches() { app -At -c "SELECT COALESCE(string_agg(quote_literal(kode), ','), '''''') FROM refbranch"; }

# reset_identity TABLE...: move each table's id sequence past the imported ids.
reset_identity() {
  local t
  for t in "$@"; do
    app -c "SELECT setval(pg_get_serial_sequence('$t', 'id'), COALESCE((SELECT max(id) FROM $t), 0) + 1, false)" >/dev/null
  done
}
