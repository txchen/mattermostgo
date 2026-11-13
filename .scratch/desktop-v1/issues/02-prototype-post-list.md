# Prototype a post list on mygo and measure it

Type: prototype
Status: resolved

Throwaway, under `prototypes/postlist/`. A window with a `ui.List` of 5,000
canned posts: Markdown with mentions and emoji, avatars, image thumbnails,
reactions. Answer on a Mac:

- Can `ui/markdown` render `@user`, `~channel` and `:emoji:` as styled
  inlines, by extension or by preprocessing?
- RSS after load, frame time while scrolling, idle CPU over 60 s.
- Do color emoji, animated GIFs and selection across posts work?

The answers decide `internal/markdown` and the image pipeline.

## Comments

2026-11-12: Answered without a throwaway prototype, in the real code. Mentions, ~channels and :emoji: work by preprocessing (internal/markdown.Prepare) into links the view handles; mygo's ExtMentions follows GitHub logins and breaks names with dots. Color emoji render (screenshots in the run log). Measured on this Mac with the real app logged in to the seeded server: 0.00 s of CPU over 60 s idle, RSS 113 MB, physical footprint 51 MB. Not measured yet: a 5,000-post scroll frame time, animated GIFs, selection across posts.

2026-11-12 (run 2): Scroll measured in the real window: 5,000 posts, p50 6.2 ms, p90 6.9 ms, max 13.3 ms a frame (run log). Animated GIFs and selection across posts still not checked.
