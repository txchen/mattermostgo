# 0001. Native UI for everything, no webview

Status: accepted, 2026-11-12

## Context

The official client embeds the Mattermost webapp. Running it in WKWebView
instead of Electron would remove the bundled Chromium. Each server would
still run the full React webapp in a WebKit content process, which is most
of the memory and CPU the project wants to save.

## Decision

The whole interface is drawn with mygo's native `ui` package. The client
talks to the server only through the REST API v4 and the WebSocket. No
window opens a webview in v1.

## Consequences

- Package size and memory are bounded by our own code.
- Webapp plugins (Boards, Playbooks, Calls, third-party) are not available.
  Each needs a native implementation or stays unsupported.
- We own every chat feature and follow server API changes ourselves.
- SSO needs a flow that does not depend on rendering the server's pages in
  the app, so it waits until we find one (see the spec's deferred list).
