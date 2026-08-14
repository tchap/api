#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

VERIFY_DIR=$(mktemp -d -t served-api-inventory-verify-XXXXXX)

go run --mod=vendor -trimpath github.com/openshift/api/payload-command/cmd/write-served-api-inventory \
  --source-dir=. \
  --go-output-dir="${VERIFY_DIR}"

diff "${VERIFY_DIR}/zz_generated_openshift.go" ./servedapis/zz_generated_openshift.go

rm -rf "${VERIFY_DIR}"
