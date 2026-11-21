#!/bin/sh
# Long-running lab job: touch a marker under the site tmp/ and sleep forever.
set -eu
MARKER="${WBR_JOB_MARKER:-tmp/job-alive}"
mkdir -p "$(dirname "$MARKER")"
echo "lab-keep pid=$$ started $(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$MARKER"
while true; do
  date -u +%Y-%m-%dT%H:%M:%SZ >>"$MARKER"
  sleep 30
done
