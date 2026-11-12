# Prototype a post list on mygo and measure it

Type: prototype
Status: open

Throwaway, under `prototypes/postlist/`. A window with a `ui.List` of 5,000
canned posts: Markdown with mentions and emoji, avatars, image thumbnails,
reactions. Answer on a Mac:

- Can `ui/markdown` render `@user`, `~channel` and `:emoji:` as styled
  inlines, by extension or by preprocessing?
- RSS after load, frame time while scrolling, idle CPU over 60 s.
- Do color emoji, animated GIFs and selection across posts work?

The answers decide `internal/markdown` and the image pipeline.
