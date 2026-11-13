//go:build !mygo_noinspector

// Development builds link the automation plugin, which listens only when
// MYGO_AUTOMATION names a socket. Release builds leave it out.
package main

import _ "github.com/egoist/mygo/plugins/automation"
