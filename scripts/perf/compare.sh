#!/bin/sh
# Runs bigscroll.sh and an idle check on one debug build of the app,
# logged in as alice against the dev server, to compare builds:
#
#   scripts/perf/compare.sh /tmp/perf-old/MMGoDebug.app > old.txt
#
# The bundle must be named MMGoDebug.app, as bigscroll.sh finds it so.
# The display must stay awake (caffeinate -u -d) for frames to present.
set -u
APPDIR=$1
SOCK=/tmp/mmperf.sock
LOG=/tmp/mmperf.log
A=${MYGO_AUTOMATION_CLI:-mygo-automation}
pkill -x 'Mattermost Go' 2>/dev/null
sleep 1
rm -f "$SOCK" "$LOG"
open -n "$APPDIR" --env MYGO_AUTOMATION=$SOCK --env MYGO_FRAME_STATS=all --stderr $LOG
sleep 8
$A open --socket $SOCK >/dev/null || exit 1
PID=$(pgrep -f 'MMGoDebug.app/Contents/MacOS' | head -1)
echo "rss at start: $(ps -o rss= -p "$PID") KB"
MMAPP_LOG=$LOG "$(dirname "$0")/bigscroll.sh"
# Idle: the window in the background for 60 s.
$A press "$($A snapshot -i | grep -F '"Town Square" key=' | head -1 | awk '{print $1}')" --settle >/dev/null 2>&1
osascript -e 'tell application "Finder" to activate' >/dev/null 2>&1
sleep 3
before=$(ps -o time= -p "$PID")
frames=$(grep -c 'mygo: frame' $LOG)
sleep 60
after=$(ps -o time= -p "$PID")
echo "idle 60 s in the background: cpu $before -> $after, frames $(( $(grep -c 'mygo: frame' $LOG) - frames )), rss $(ps -o rss= -p "$PID") KB"
kill "$PID"
