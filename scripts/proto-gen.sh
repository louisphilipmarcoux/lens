#!/usr/bin/env bash
set -euo pipefail

PROTO_DIR="internal/common/proto"
GEN_DIR="${PROTO_DIR}/gen"

mkdir -p "${GEN_DIR}"

protoc \
  --go_out="${GEN_DIR}" --go_opt=paths=source_relative \
  --go-grpc_out="${GEN_DIR}" --go-grpc_opt=paths=source_relative \
  -I "${PROTO_DIR}" \
  "${PROTO_DIR}"/*.proto

echo "Protobuf generation complete."
