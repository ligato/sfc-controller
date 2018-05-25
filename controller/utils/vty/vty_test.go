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
	"fmt"
	"testing"

	"github.com/ligato/sfc-controller/controller/utils/vty/ssh"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
)

func TestSSH(t *testing.T) {
	var log = logrus.DefaultLogger()
	log.SetLevel(logging.DebugLevel)

	conn, err := ssh.Connect("10.10.10.10", 22, "cisco", "cisco")
	if err != nil {
		t.Error(err)
		return
	}
	defer conn.Close()

	sess, err := NewSession(conn)
	if err != nil {
		t.Error(err)
		return
	}
	defer sess.Close()

	output, err := sess.ExecCMD(
		"conf t",
		"ip route 1.2.3.4 255.255.255.255 4.3.2.1",
		"exit")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(output)
}
