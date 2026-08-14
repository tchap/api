#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

go run --mod=vendor -trimpath github.com/openshift/api/payload-command/cmd/write-served-api-inventory \
  --source-dir=. \
  --go-output-dir=./servedapis
