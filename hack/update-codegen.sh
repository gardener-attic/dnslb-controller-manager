#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2017 The Kubernetes Authors.
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=""$(readlink -f "$(dirname ${0})/..")""
source "$SCRIPT_ROOT/build/settings.src"

CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  $PKGPATH/pkg/client \
  $PKGPATH/pkg/apis \
  loadbalancer:v1beta1 \
  --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

# To use your own boilerplate text use:
#   --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt