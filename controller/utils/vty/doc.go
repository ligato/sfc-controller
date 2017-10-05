// Package vty provides the ability to execute remote commands (e.g. CLI) on an existing
// remote connection (any transport that implements the Connector interface).
// To execute remote commands, a new configuration session needs to be opened:
//
//  sess, err := NewSession(conn)
//  // handle error!
//  defer sess.Close()
//
// Then remote commands can be executed using the ExecCMD method of the session:
//
//  output, err := sess.ExecCMD("show ver")
//  // handle error!
//
// Each command is a string that will be sent as the input via the open connection on a separate line.
// After sending the command (or multiple commands in one ExecCMD call), all the output will be read
// and returned, until the shell prompt is read from the connection or until the timeout expires.
package vty
