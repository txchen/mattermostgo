# Mattermost Go and the official desktop client

Measured on 2026-11-13 on one Mac (Apple M1, macOS 27.2), with
Mattermost Go 0.4.0 and the official Mattermost Desktop 6.3.2.

| | Mattermost Go 0.4.0 | Mattermost Desktop 6.3.2 |
|---|---:|---:|
| Download, universal dmg | **9.3 MB** | 231.8 MB |
| Download, Apple Silicon only | | 129.4 MB |
| App on disk | **30 MB** (universal) | 296 MB (Apple Silicon only) |
| Processes while running | **1** | 11 |
| Memory (physical footprint) | **98 MB**, logged in, a channel open | 338 MB on its first screen, no server added |
| Memory after a minute in the background | **61 MB** | |
| CPU in the background, over 60 s | **under 0.01 s** | 1.0 s |

## What was measured, and what was not

- **Download** sizes are those of the release files on GitHub. **App on
  disk** is `du -sh` of the app bundle. Mattermost Go is universal (Apple
  Silicon and Intel in one app); the official client was the Apple
  Silicon build, the smaller of its two.
- **Memory** is the physical footprint macOS reports (`footprint`), summed
  over every process of the app. It counts memory shared between
  processes once, unlike adding up their RSS (1,072 MB for the official
  client's 11 processes).
- The two are not in the same state, and the difference favours the
  official client. The test server has no webapp, which the official
  client needs to log in, so it was measured on its first screen with no
  server added. Mattermost Go was measured logged in to a server, with a
  channel and its posts loaded. A logged-in official client also loads
  the whole webapp into its renderer, so its real use is higher than
  shown here. Measure both on your own server for a like-for-like number.
- Mattermost Go was a development build, which also carries the
  automation plugin of the tests; the release build carries less.
- **CPU in the background** is the CPU time `ps` reports, before and
  after 60 s with another app in front.

## How to measure again

```sh
# The official client
gh release download v6.3.2 -R mattermost/desktop -p 'mattermost-desktop-6.3.2-mac-arm64.zip'
ditto -x -k mattermost-desktop-6.3.2-mac-arm64.zip . && du -sh Mattermost.app
open -n Mattermost.app; sleep 20
footprint $(pgrep -f 'Mattermost.app/' | sed 's/^/-p /')        # Summary Footprint
ps -axo time,command | grep 'Mattermost.app/'                    # CPU time, again 60 s later

# Mattermost Go, logged in
footprint -p $(pgrep -x 'Mattermost Go')
ps -o time= -p $(pgrep -x 'Mattermost Go')
```

Frame times and memory while scrolling 5,000 posts, and how they moved
since 0.1.1, are in the [run log](../.scratch/desktop-v1/run-log.md).
