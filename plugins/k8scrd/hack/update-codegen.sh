#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
set -x
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/../../..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

# generate the code with:
${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/ligato/sfc-controller/plugins/k8scrd/pkg/client \
  github.com/ligato/sfc-controller/plugins/k8scrd/pkg/apis \
  sfccontroller:v1alpha1 \
  --go-header-file ${SCRIPT_ROOT}/plugins/k8scrd/hack/custom-boilerplate.go.txt

# generate sfc-controller plugin model deepcopy
$GOPATH/bin/deepcopy-gen \
  --input-dirs github.com/ligato/sfc-controller/plugins/controller/model \
  --bounding-dirs github.com/ligato/sfc-controller/plugins/controller/model \
  --go-header-file ${SCRIPT_ROOT}/plugins/k8scrd/hack/custom-boilerplate.go.txt \
  -O zz_generated.deepcopy-fix-me

# Fix the package name in the model deepcopy
sed 's/package model/package controller/' \
   < plugins/controller/model/zz_generated.deepcopy-fix-me.go \
   > plugins/controller/model/zz_generated.deepcopy.go
rm plugins/controller/model/zz_generated.deepcopy-fix-me.go