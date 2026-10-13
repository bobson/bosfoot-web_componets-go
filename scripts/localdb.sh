#!/usr/bin/env bash
# Run a command against the LOCAL sandbox Postgres (tmp/pg) instead of the live
# Aiven DB — for testing migrations, imports and test orders without touching
# production. The sandbox uses TLS with its own CA, so the app's normal
# verify-ca connection works unchanged.
#
#   bash scripts/localdb.sh init                  # create it (once; then load data:
#                                                 #   dbimport, addbrands, shoeimport -commit)
#   bash scripts/localdb.sh start                 # start the sandbox server
#   bash scripts/localdb.sh stop                  # stop it
#   bash scripts/localdb.sh go run ./cmd/dbimport # any command, pointed at it
#   bash scripts/localdb.sh go run main.go        # local site on the sandbox
#
# Exported vars win over .env (godotenv doesn't override), so the command sees
# the sandbox DATABASE_URL. Created once under tmp/pg (gitignored) by `init`.
# (air's tmp_dir is tmp/air, not tmp — its clean_on_exit would wipe tmp/pg.)
set -euo pipefail
cd "$(dirname "$0")/.."
P=tmp/pg

if [[ ${1:-} == init ]]; then
	[[ -e $P ]] && { echo "$P already exists — delete it first to recreate" >&2; exit 1; }
	mkdir -p "$P"
	openssl rand -hex 16 >"$P/pw"
	initdb -D "$P/data" -U bosfoot --pwfile="$P/pw" -A scram-sha-256 >/dev/null
	# Own CA + server cert so the app's verify-ca TLS connection works unchanged.
	openssl req -x509 -newkey rsa:2048 -nodes -days 3650 -subj "/CN=bosfoot sandbox CA" \
		-keyout "$P/ca.key" -out "$P/ca.crt" 2>/dev/null
	openssl req -newkey rsa:2048 -nodes -subj "/CN=localhost" \
		-keyout "$P/data/server.key" -out "$P/server.csr" 2>/dev/null
	openssl x509 -req -in "$P/server.csr" -CA "$P/ca.crt" -CAkey "$P/ca.key" -CAcreateserial \
		-days 3650 -out "$P/data/server.crt" 2>/dev/null
	chmod 600 "$P/data/server.key"
	cat >>"$P/data/postgresql.conf" <<CONF
port = 5433
listen_addresses = 'localhost'
unix_socket_directories = '$PWD/$P'
ssl = on
CONF
	pg_ctl -D "$P/data" -l "$P/log" -w start >/dev/null
	PGPASSWORD=$(cat "$P/pw") createdb -h localhost -p 5433 -U bosfoot bosfoot
	echo "sandbox created at $P (port 5433). Load data: bash $0 go run ./cmd/dbimport"
	exit 0
fi
[[ -d $P/data ]] || { echo "no sandbox at $P — see project notes to create it" >&2; exit 1; }

case "${1:-}" in
start) exec pg_ctl -D "$P/data" -l "$P/log" -w start ;;
stop) exec pg_ctl -D "$P/data" -w stop ;;
"") echo "usage: localdb.sh start|stop|<command…>" >&2; exit 2 ;;
esac

pg_ctl -D "$P/data" status >/dev/null 2>&1 || pg_ctl -D "$P/data" -l "$P/log" -w start >/dev/null
export DATABASE_URL="postgres://bosfoot:$(cat "$P/pw")@localhost:5433/bosfoot?sslmode=require"
export PG_CA_CERT="$P/ca.crt"
exec "$@"
