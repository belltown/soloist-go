#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "${SCRIPT_DIR}/.env"

/usr/local/bin/soloist --device-name "solo-go" --api-key "${SOLOIST_API_KEY}" --ws "127.0.0.1:${SOLOIST_PORT}"
