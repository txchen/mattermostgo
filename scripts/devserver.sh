#!/bin/sh
# Builds and starts a local Mattermost server for development, without
# Docker: PostgreSQL from Homebrew and the server built from source.
#
#   scripts/devserver.sh start   # build if needed, start, seed once
#   scripts/devserver.sh stop
#
# Everything lives under $MM_DEV (default /tmp/mmserver). The server
# listens on http://localhost:8065; every seeded user's password is
# Password1!, and $MM_DEV/mfa-secret holds mfauser's MFA secret.
set -eu

DEV=${MM_DEV:-/tmp/mmserver}
PG=${PG_PREFIX:-/opt/homebrew/opt/postgresql@17}/bin
SERVER_VERSION=${MM_SERVER_VERSION:-v8.0.0-20261112195927-9e3a32bc2092}
PUBLIC_VERSION=${MM_PUBLIC_VERSION:-v0.6.2-0.20261112195927-9e3a32bc2092}
ROOT=$(cd "$(dirname "$0")/.." && pwd)

start() {
	mkdir -p "$DEV"
	if [ ! -x "$DEV/mattermost" ]; then
		dir=$(GOFLAGS=-mod=mod go mod download -json "github.com/mattermost/mattermost/server/v8@$SERVER_VERSION" | sed -n 's/.*"Dir": "\(.*\)".*/\1/p')
		rm -rf "$DEV/src" && cp -R "$dir" "$DEV/src" && chmod -R u+w "$DEV/src"
		(cd "$DEV/src" &&
			GOSUMDB=off GOFLAGS=-mod=mod go mod edit -require="github.com/mattermost/mattermost/server/public@$PUBLIC_VERSION" &&
			GOSUMDB=off GOFLAGS=-mod=mod go build -o "$DEV/mattermost" ./cmd/mattermost)
	fi
	if [ ! -d "$DEV/pgdata" ]; then
		"$PG/initdb" -D "$DEV/pgdata" -U mmuser --auth=trust >"$DEV/initdb.log"
	fi
	"$PG/pg_ctl" -D "$DEV/pgdata" -o "-p 5433 -k $DEV -c listen_addresses=localhost" -l "$DEV/pg.log" -w start
	"$PG/createdb" -h localhost -p 5433 -U mmuser mattermost 2>/dev/null && fresh=1 || fresh=0
	mkdir -p "$DEV/data" "$DEV/client" && echo '<html></html>' >"$DEV/client/root.html"
	cd "$DEV/src"
	MM_SQLSETTINGS_DRIVERNAME=postgres \
	MM_SQLSETTINGS_DATASOURCE='postgres://mmuser@localhost:5433/mattermost?sslmode=disable' \
	MM_SERVICESETTINGS_SITEURL=http://localhost:8065 \
	MM_SERVICESETTINGS_LISTENADDRESS=:8065 \
	MM_SERVICESETTINGS_ENABLEMULTIFACTORAUTHENTICATION=true \
	MM_SERVICESETTINGS_ENABLELINKPREVIEWS=true \
	MM_SERVICESETTINGS_ENABLEBOTACCOUNTCREATION=true \
	MM_SERVICESETTINGS_ENABLEUSERACCESSTOKENS=true \
	MM_SERVICESETTINGS_ENABLEINCOMINGWEBHOOKS=true \
	MM_SERVICESETTINGS_ENABLEOUTGOINGWEBHOOKS=true \
	MM_SERVICESETTINGS_ENABLECOMMANDS=true \
	MM_SERVICESETTINGS_ENABLEPOSTUSERNAMEOVERRIDE=true \
	MM_SERVICESETTINGS_ALLOWEDUNTRUSTEDINTERNALCONNECTIONS='localhost 127.0.0.1' \
	MM_FILESETTINGS_DIRECTORY="$DEV/data" \
	MM_TEAMSETTINGS_ENABLEOPENSERVER=true \
	MM_RATELIMITSETTINGS_ENABLE=false \
	MM_PLUGINSETTINGS_ENABLE=false \
	MM_EMAILSETTINGS_SENDEMAILNOTIFICATIONS=false \
	MM_LOGSETTINGS_ENABLECONSOLE=false \
	MM_LOGSETTINGS_FILELOCATION="$DEV" \
	nohup "$DEV/mattermost" server --config "$DEV/config.json" >"$DEV/server.out" 2>&1 &
	echo $! >"$DEV/server.pid"
	until curl -sf http://localhost:8065/api/v4/system/ping >/dev/null; do sleep 1; done
	if [ "$fresh" = 1 ]; then
		(cd "$ROOT" && go run ./scripts/seed -url http://localhost:8065 -mfa "$DEV/mfa-secret")
	fi
	echo "Mattermost is up at http://localhost:8065"
}

stop() {
	[ -f "$DEV/server.pid" ] && kill "$(cat "$DEV/server.pid")" 2>/dev/null || true
	rm -f "$DEV/server.pid"
	"$PG/pg_ctl" -D "$DEV/pgdata" -w stop 2>/dev/null || true
}

case "${1:-}" in
start) start ;;
stop) stop ;;
*) echo "usage: $0 start|stop" >&2; exit 2 ;;
esac
