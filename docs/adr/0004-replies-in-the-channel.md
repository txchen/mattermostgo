# 0004. Replies show in the channel, and unread counts include them

Status: superseded by [0005](0005-collapsed-reply-threads.md), 2026-11-12

## Context

Mattermost has two ways to show replies. Without Collapsed Reply Threads
(CRT), replies appear in the channel's post list, and a channel's unread
and mention counts include them (`total_msg_count`, `msg_count`,
`mention_count`). With CRT, the channel shows root posts only, replies
live in a Threads view, and the counts are the `_root` variants. CRT is
on by default on recent servers, but each client says which mode it
supports: `POST /channels/members/me/view` takes
`collapsed_threads_supported`, and the post list endpoints take
`collapsedThreads`.

The Threads view is on the spec's deferred list.

## Decision

v1 is a non-CRT client. The post list shows replies inline, marked
"replied in a thread", which opens the RHS. The store counts unread and
mentions from the non-root fields, and the client sends
`collapsed_threads_supported: false` when it marks a channel viewed.

## Consequences

- No reply is hidden from a user who has no Threads view.
- Counts agree with what the post list shows.
- A user who follows threads in the webapp sees them only in their
  channels here. Moving to CRT later means the `_root` counts, the
  Threads view and thread follow state, together.
