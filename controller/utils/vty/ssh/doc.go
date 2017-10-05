// Package ssh provides the API for connecting to a SSH node. Once connected, reader and
// writer interface can be retrieved for the given connection using the GetVTY method.
// The reader and writer can be later used to receive/send data from/to the connected host.
// The package currently supports only the password authentication. Example of the usage:
//
//  conn, err := ssh.Connect("10.10.10.10", 22, "cisco", "cisco")
//  // handle error!
//  defer conn.Close()
//
//  r, w, err := conn.GetVTY()
//
// This package can be used as a transport for the vty package, that provides the API
// to execute remote commands on a VTY connection.
package ssh
