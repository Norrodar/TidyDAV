#!/bin/sh
# Drop privileges to PUID:PGID (LinuxServer.io-style) so files in the data
# volume are owned correctly without running the app as root.
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

DB_PATH=${TIDYDAV_DB_PATH:-/data/tidydav.db}
DATA_DIR=$(dirname "$DB_PATH")

mkdir -p "$DATA_DIR"
chown -R "$PUID:$PGID" "$DATA_DIR" 2>/dev/null || true

# su-exec accepts numeric uid:gid directly, so no passwd entry is required.
exec su-exec "$PUID:$PGID" /tidydav "$@"
