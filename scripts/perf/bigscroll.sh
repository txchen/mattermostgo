#!/bin/sh
# Opens ~big, loads every page by scrolling up, then scrolls the whole
# channel down and up, and summarizes the frame stats of each phase.
A=${MYGO_AUTOMATION_CLI:-mygo-automation}
PID=$(pgrep -f 'MMGoDebug.app/Contents/MacOS' | head -1)
mark() { wc -l < ${MMAPP_LOG:-/tmp/mmapp.log} | tr -d ' '; }
$A press $($A snapshot -i | grep '"Big Channel"' | awk '{print $1}') --settle >/dev/null 2>&1
sleep 2; osascript -e "tell application \"System Events\" to set frontmost of (first process whose unix id is $PID) to true" >/dev/null 2>&1
echo "rss after opening: $(ps -o rss= -p $PID) KB"
m0=$(mark); t=$(date +%s)
n=0; while [ $n -lt 600 ]; do $A wheel 900 400 0 -1500 >/dev/null 2>&1; n=$((n+1)); done
sleep 1
echo "load to top: $(( $(date +%s)-t ))s, rss $(ps -o rss= -p $PID) KB, top shows Post 1: $($A snapshot | grep -c 'Post 1: a plain')"
m1=$(mark)
n=0; while [ $n -lt 300 ]; do $A wheel 900 400 0 600 >/dev/null 2>&1; n=$((n+1)); done
n=0; while [ $n -lt 300 ]; do $A wheel 900 400 0 -600 >/dev/null 2>&1; n=$((n+1)); done
m2=$(mark)
echo "rss after scrolling: $(ps -o rss= -p $PID) KB"
for phase in "loading:$m0:$m1" "scrolling:$m1:$m2"; do
  name=${phase%%:*}; rest=${phase#*:}; a=${rest%%:*}; b=${rest#*:}
  sed -n "$((a+1)),${b}p" ${MMAPP_LOG:-/tmp/mmapp.log} | python3 "$(dirname "$0")/framestats.py" "$name"
done
