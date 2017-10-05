// Package iosxecfg provides a configuration interface to the IOS XE Router using the high-level protobuf API.
// The connection to the router can be based on an existing VTY session (which allows to use any type
// of the transport that the VTY package offers), or created from scratch via SSH using the NewSSHSession() method:
//
//  s, err := NewSSHSession("10.195.94.48", 22, "cisco", "cisco")
//  // check error!
//  defer s.Close()
//
// Once there is a configuration session open, one can use its method to configure the router, e.g.:
//
//  err = s.AddStaticRoute(&iosxe.StaticRoute{
//  	DstAddress:     "1.2.3.4/32",
//  	NextHopAddress: "8.8.8.8",
//  })
//
package iosxecfg
