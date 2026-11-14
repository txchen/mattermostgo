#!/bin/sh
# Drives the real app window with mygo's automation plugin, against the
# server of scripts/devserver.sh, and checks each interaction on the
# server's side. The app must be a development build started with
# MYGO_AUTOMATION=$SOCK and logged in as alice (see the README).
#
#   scripts/realwindow.sh
#
# Prints PASS or FAIL per check and exits 1 if any failed.
set -u
A=${MYGO_AUTOMATION_CLI:-mygo-automation}
SOCK=${SOCK:-/tmp/mmapp.sock}
URL=${MM_URL:-http://localhost:8065}
failed=0
# RUN makes every message of this run its own, so that no check passes on
# what an earlier run left on the server.
RUN=$(date +%s)
APP=${MM_APP:-}

$A open --socket "$SOCK" >/dev/null || exit 1

pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $2"; failed=1; }
ref() { $A snapshot -i | grep -F -- "$1" | head -1 | awk '{print $1}'; }
token() { curl -s -i -X POST "$URL/api/v4/users/login" -d "{\"login_id\":\"$1\",\"password\":\"Password1!\"}" | grep -i '^token:' | awk '{print $2}' | tr -d '\r'; }
ALICE=$(token alice)
BOB=$(token bob)
api() { curl -s -H "Authorization: Bearer $1" "$URL/api/v4$2"; }
post_as() { curl -s -o /dev/null -H "Authorization: Bearer $1" -X POST "$URL/api/v4/posts" -d "$2"; }
channel_id() { api "$ALICE" "/teams/name/eng/channels/name/$1" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])'; }
# last <channel id> <field>: a field of the channel's newest post.
last() { api "$ALICE" "/channels/$1/posts?per_page=1" | python3 -c "import json,sys; l=json.load(sys.stdin); p=l['posts'][l['order'][0]]; print(json.dumps(p.get('$2')) if '$2' in ('file_ids','metadata') else p.get('$2',''))"; }
open_channel() { $A press "$(ref "\"$1\" key=")" --settle >/dev/null; sleep 0.5; }
send() { $A type "$1" --settle >/dev/null; $A key Enter >/dev/null; sleep 1; }
TOWN=$(channel_id town-square)
DEV=$(channel_id dev)

open_channel "Town Square"
$A focus "$(ref '"Message"')" >/dev/null
msg="real window $RUN"
send "$msg **bold**"
[ "$(last "$TOWN" message)" = "$msg **bold**" ] && pass "Enter sends" || fail "Enter sends" "$(last "$TOWN" message)"

$A type "line one $RUN" --settle >/dev/null; $A key Shift+Enter >/dev/null; send "line two"
[ "$(last "$TOWN" message)" = "line one $RUN
line two" ] && pass "Shift+Enter breaks the line" || fail "Shift+Enter breaks the line" "$(last "$TOWN" message)"

lost=0
for i in 1 2 3 4 5; do
	$A type "quick $RUN $i" --settle >/dev/null; $A key Enter >/dev/null; $A type "next" --settle >/dev/null; sleep 0.3
	$A snapshot | grep -F 'textField] "Message"' | grep -q 'value="next"' || lost=$((lost+1))
	$A key CmdOrCtrl+A >/dev/null; $A key Backspace >/dev/null; sleep 0.2
done
[ $lost = 0 ] && pass "text typed right after Enter stays" || fail "text typed right after Enter stays" "lost $lost of 5"

$A type "@bo" --settle >/dev/null; sleep 0.5
if $A snapshot | grep -q '"@bob"'; then pass "@ suggests users"; else fail "@ suggests users" "no @bob"; fi
$A key Enter >/dev/null; send "hi $RUN"
[ "$(last "$TOWN" message)" = "@bob hi $RUN" ] && pass "Enter completes the mention" || fail "Enter completes the mention" "$(last "$TOWN" message)"

$A type ":tad" --settle >/dev/null
# Tab completes only once the suggestion shows; a fixed sleep raced it.
n=0; until $A snapshot | grep -q ':tada:' || [ $n -ge 20 ]; do sleep 0.1; n=$((n+1)); done
$A key Tab >/dev/null; send "done $RUN"
[ "$(last "$TOWN" message)" = ":tada: done $RUN" ] && pass "Tab completes an emoji" || fail "Tab completes an emoji" "$(last "$TOWN" message)"

$A key Up >/dev/null; sleep 0.5
if $A snapshot | grep -q 'Enter to save'; then pass "Up edits the last post"; else fail "Up edits the last post" "no edit box"; fi
$A key CmdOrCtrl+A >/dev/null; $A type "edited in place $RUN" --settle >/dev/null; $A key Enter >/dev/null; sleep 1
[ "$(last "$TOWN" message)" = "edited in place $RUN" ] && [ "$(last "$TOWN" edit_at)" != "0" ] && pass "the edit saves" || fail "the edit saves" "$(last "$TOWN" message)"

$A press "$(ref '"Bold"')" >/dev/null; $A type "strong$RUN" --settle >/dev/null; $A key Enter >/dev/null; sleep 1
[ "$(last "$TOWN" message)" = "**strong$RUN**" ] && pass "the Bold button wraps the text" || fail "the Bold button wraps the text" "$(last "$TOWN" message)"

open_channel "Town Square"
$A key CmdOrCtrl+K >/dev/null; sleep 0.5; $A type "random" --settle >/dev/null; $A key Enter >/dev/null; sleep 1
heading() { $A snapshot | grep ' channel menu"' | head -1 | sed 's/.*\] "\(.*\) channel menu".*/\1/'; }
[ "$(heading)" = "Random" ] && pass "Cmd+K opens a channel" || fail "Cmd+K opens a channel" "header $(heading)"
$A key Alt+Up >/dev/null; sleep 1
[ "$(heading)" = "Off-Topic" ] && pass "Option+Up opens the channel above" || fail "Option+Up opens the channel above" "header $(heading)"
$A key "CmdOrCtrl+[" >/dev/null; sleep 1
[ "$(heading)" = "Random" ] && pass "Cmd+[ goes back" || fail "Cmd+[ goes back" "header $(heading)"

open_channel "Developers"
post_as "$BOB" "{\"channel_id\":\"$DEV\",\"message\":\"react to me $(date +%s)\"}"
sleep 1
target=$($A snapshot | grep -F 'react to me' | tail -1 | awk '{print $1}')
$A pointer move "$target" >/dev/null; sleep 0.5
$A press "$(ref '"React with :+1:"')" --settle >/dev/null; sleep 1
reacted=$(last "$DEV" metadata | python3 -c 'import json,sys; m=json.loads(sys.stdin.read() or "null") or {}; print(",".join(r["emoji_name"] for r in m.get("reactions", [])))')
[ "$reacted" = "+1" ] && pass "hover toolbar reacts" || fail "hover toolbar reacts" "reactions: $reacted"

$A pointer move "$target" >/dev/null; sleep 0.3
$A press "$(ref '"Reply"')" --settle >/dev/null; sleep 1
if $A snapshot | grep -q '"Close thread"'; then pass "Reply opens the thread"; else fail "Reply opens the thread" "no RHS"; fi
root=$(last "$DEV" id)
$A focus "$($A snapshot -i | grep -F '"Message"' | tail -1 | awk '{print $1}')" >/dev/null
send "a reply from the RHS $RUN"
[ "$(last "$DEV" root_id)" = "$root" ] && [ "$(last "$DEV" message)" = "a reply from the RHS $RUN" ] && pass "the RHS replies in the thread" || fail "the RHS replies in the thread" "root $(last "$DEV" root_id)"
$A press "$(ref '"Close thread"')" >/dev/null

python3 -c "
import zlib,struct
def chunk(t,d): return struct.pack('>I',len(d))+t+d+struct.pack('>I',zlib.crc32(t+d)&0xffffffff)
raw=b''.join(b'\x00'+b''.join(bytes([x*4%256,y*4%256,160]) for x in range(64)) for y in range(64))
open('/tmp/paste.png','wb').write(b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',64,64,8,2,0,0,0))+chunk(b'IDAT',zlib.compress(raw))+chunk(b'IEND',b''))"
osascript -e 'set the clipboard to (read (POSIX file "/tmp/paste.png") as «class PNGf»)'
$A focus "$(ref '"Message"')" >/dev/null
$A key CmdOrCtrl+V >/dev/null; sleep 2
if $A snapshot | grep -q 'image.png'; then pass "pasting an image attaches it"; else fail "pasting an image attaches it" "no chip"; fi
send "pasted $RUN"
[ "$(last "$DEV" message)" = "pasted $RUN" ] && [ "$(last "$DEV" file_ids)" != "[]" ] && [ "$(last "$DEV" file_ids)" != "null" ] && pass "the pasted image posts" || fail "the pasted image posts" "$(last "$DEV" file_ids)"

open_channel "Town Square"
# open -a hands the link to this build, as the system does for the app
# that registered the scheme (Info.plist CFBundleURLSchemes).
if [ -n "$APP" ]; then open -a "$APP" "mattermost://localhost:8065/eng/channels/random"; else open "mattermost://localhost:8065/eng/channels/random"; fi
sleep 2
[ "$(heading)" = "Random" ] && pass "a mattermost:// link opens its channel" || fail "a mattermost:// link opens its channel" "header $(heading)"

exit $failed
