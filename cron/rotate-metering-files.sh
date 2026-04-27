#!/bin/bash
set -euo pipefail

SRC="/opt/optical-probe-reader/csv/archive"
DST="/home/pi/Metering_files"
LOG="/var/log/rotate-metering-files.log"

mkdir -p "$DST"

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Starting metering file rotation" >> "$LOG"

# Move metering CSV files
find "$SRC" -maxdepth 1 -type f -name 'metering_*.csv' -exec mv -f {} "$DST"/ \;

# Delete destination metering files older than 14 days
find "$DST" -maxdepth 1 -type f -name 'metering_*.csv' -mtime +14 -delete

# Log destination folder size
SIZE=$(du -sh "$DST" | awk '{print $1}')
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Done. Destination size: $SIZE" >> "$LOG"