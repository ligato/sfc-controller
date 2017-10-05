// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vty

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ligato/cn-infra/logging/logroot"
)

const (
	cmdTimeout        = 3 * time.Second
	disconnectTimeout = 2 * cmdTimeout
)

// Connector provides the connection to VTY via various transports.
type Connector interface {
	// GetVTY returns the interface to VTY for reading and writing.
	GetVTY() (io.Reader, io.Writer, error)

	// Close closes the VTY connection.
	Close()
}

// Session is a VTY session handle.
type Session struct {
	conn      Connector
	r         io.Reader
	w         io.Writer
	rChan     chan string
	closeChan chan bool
	errChan   chan error
}

var log = logroot.StandardLogger()

// NewSession returns a new VTY session opened on provided connector.
// The session is supposed to be closed via Close().
func NewSession(conn Connector) (*Session, error) {
	var err error
	s := &Session{conn: conn}

	// get VTY reader and writer
	s.r, s.w, err = s.conn.GetVTY()
	if err != nil {
		return nil, err
	}

	// start reading from VTY via channel
	s.rChan = make(chan string)
	s.closeChan = make(chan bool, 1)
	s.errChan = make(chan error, 1)
	go s.readLines()

	// perform the first communication attempt - an empty command
	fmt.Fprintln(s.w, "")
	_, err = readUntilShell(s.rChan, s.errChan)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Close closes the VTY session.
func (s *Session) Close() {
	s.closeChan <- true
}

// ExecCMD executes provided commands on the VTY session.
func (s *Session) ExecCMD(cmds ...string) (response string, err error) {

	// execute the commands
	for _, cmd := range cmds {
		fmt.Fprintln(s.w, cmd)
	}

	// write a new line to force the shell appear in the output
	fmt.Fprintln(s.w, "")

	// read the output until the shell appears
	return readUntilShell(s.rChan, s.errChan)
}

// readUntilShell reads the output from the read channel until the shell appears in the output, or timeout is expired.
func readUntilShell(rChan chan string, errChan chan error) (string, error) {
	var buffer bytes.Buffer
	var totalTime time.Duration
	active := false // used to track whether some output is just being generated

	for {
		select {
		case str := <-rChan:
			// a line has been read

			if strings.HasSuffix(str, "#") {
				// shel found, consider the output as complete
				return buffer.String(), nil
			}
			if str != "" {
				// non-empty string read, transition to the active state
				active = true
			}
			// append the output to the buffer
			buffer.WriteString(str + "\n")

		case <-time.After(cmdTimeout):
			// nothing read within the timeout
			totalTime += cmdTimeout

			if active {
				// no more data for some time, but there was some activity before, consider the output as complete
				log.Warn("No more response from the router within the timeout interval, considering the received output as complete.")
				return buffer.String(), nil
			} else if totalTime >= disconnectTimeout {
				// no data from the router at all, return disconnected error
				log.Error("No response from the router, disconnected?")
				return "", errors.New("no response from the router, disconnected")
			}

		case err := <-errChan:
			// error occurred
			log.Error(err)
			return "", err
		}
	}

	return "", nil
}

// readLines reads the lines from provided reader and sends them into the provided channel.
func (s *Session) readLines() {
	bReader := bufio.NewReader(s.r)
	for {
		select {
		case <-s.closeChan:
			// close requested, return
			return

		default:
			// read from the reader
			line, err := bReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					log.Debug("VTY connection closed by the remote side.")
				} else {
					error := fmt.Errorf("error by reading from VTY: %s", err)
					log.Error(error)
					s.errChan <- error
				}
				return
			}
			str := strings.Trim(line, " \r\n")
			log.Debug("CLI: ", str)

			// write the read string to the channel
			s.rChan <- str
		}
	}
}
