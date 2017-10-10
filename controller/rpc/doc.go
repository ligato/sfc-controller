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

// Package rpc contains SFC Controller HTTP REST handlers implementation.
// The model for the controller in
// ligato/sfc-controller/controller/model drives the REST interface.  The
// model is described in the protobuf file.  Each of the entites like hosts,
// external routers, and SFC's can be configrued (CRUDed) via REST calls.
package rpc
