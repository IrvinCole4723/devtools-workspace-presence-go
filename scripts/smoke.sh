#!/bin/sh
set -eu

base_url="${SERVICE_URL:-http://localhost:8080}"
curl --fail --silent --show-error "$base_url/healthz"
curl --fail --silent --show-error "$base_url/online"
