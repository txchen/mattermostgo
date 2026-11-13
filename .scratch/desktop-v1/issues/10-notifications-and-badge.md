# Notifications and Dock badge

Type: task
Status: open
Blocked by: 05

Spec feature 10.

## Comments

2026-11-12: Code done (shouldNotify, Dock badge, click opens the channel), unit tested. Not verified: a real macOS notification needs the signed bundle and permission, which no test here covers.

2026-11-12 (run 2): The app reaches Notification.Show and macOS refuses for the /tmp debug bundle without asking; the app now tells the user once. Still needs a look with a signed build.
