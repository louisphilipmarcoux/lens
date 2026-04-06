#!/usr/bin/env bash
set -euo pipefail

# Generate Go bindings from BPF C programs using bpf2go.
# Requires: clang, llvm, linux-headers, bpf2go
#
# Install bpf2go:
#   go install github.com/cilium/ebpf/cmd/bpf2go@latest
#
# This script must be run on Linux with kernel headers available.

BPF_DIR="internal/agent/collector/ebpf/bpf"
OUT_DIR="internal/agent/collector/ebpf"

# Generate vmlinux.h if not present.
if [ ! -f "${BPF_DIR}/vmlinux.h" ]; then
    echo "Generating vmlinux.h from BTF..."
    bpftool btf dump file /sys/kernel/btf/vmlinux format c > "${BPF_DIR}/vmlinux.h"
fi

echo "Generating BPF Go bindings..."

cd "${OUT_DIR}"

bpf2go -cc clang -cflags "-O2 -g -Wall -I ../../../${BPF_DIR}" \
    tcpRetransmit "../../../${BPF_DIR}/tcp_retransmit.c" -- -I/usr/include

bpf2go -cc clang -cflags "-O2 -g -Wall -I ../../../${BPF_DIR}" \
    syscallLatency "../../../${BPF_DIR}/syscall_latency.c" -- -I/usr/include

bpf2go -cc clang -cflags "-O2 -g -Wall -I ../../../${BPF_DIR}" \
    bioLatency "../../../${BPF_DIR}/bio_latency.c" -- -I/usr/include

echo "BPF generation complete."
