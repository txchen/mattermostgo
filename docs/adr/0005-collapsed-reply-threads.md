# 0005. Collapsed Reply Threads, as the webapp

Status: accepted, 2026-11-12. Supersedes [0004](0004-replies-in-the-channel.md).

## Context

ADR 0004 showed replies inside their channel, as Mattermost does with
Collapsed Reply Threads (CRT) off. Users compared the client with the
official one, which servers run with CRT on: a channel shows root posts
only, each with a summary of its thread, and replies open in the RHS. A
long bot reply in the middle of a channel read as wrong.

## Decision

The client is a CRT client. It asks for posts with
`collapsedThreads=true`, marks channels viewed with
`collapsed_threads_supported: true`, and reads the root counts
(`total_msg_count_root`, `msg_count_root`, `mention_count_root`). The
store never puts a reply in a channel timeline: a reply adds to its
root's reply count, last reply time and participants, and to the thread
when it is loaded. Root posts show who replied, how many replies and
when the last came; a click opens the thread.

## Consequences

- Channels read as they do in the official client.
- A mention inside a thread does not count on the channel; the official
  client counts it in its Threads view, which this client does not have
  yet (deferred). Desktop notifications still come for it.
- Following threads and the Threads view are the next step on this path.
