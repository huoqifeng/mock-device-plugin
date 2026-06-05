#!/bin/bash
# Save this file as /opt/mock-xpu-cmd.sh
# Make it executable: chmod +x /opt/mock-xpu-cmd.sh
#
# Mock script for XPU device commands (NVIDIA GPU & Huawei Ascend NPU).
# This script intercepts nvidia-smi, npu-smi, and crictl commands
# to simulate device debugging scenarios described in mock-xpu-cmd.md.
# Only READ-only commands are mocked; WRITE commands (reset, kill) fall
# through to the real binaries.
#
# Environment variables for configuration (read from /etc/mock-xpu.env if exists):
#   MOCK_NVIDIA_PIDS          - Comma-separated "pid:process_name:used_memory" for GPU processes
#   MOCK_NPU_PIDS             - Comma-separated "pid:type:used_memory" for NPU processes
#   MOCK_CONTAINER_ID         - Container ID to return from crictl inspectp mock

# Source config file if exists
if [ -f /etc/mock-xpu.env ]; then
    source /etc/mock-xpu.env
fi

ARGS="$*"

# --- Helper: default GPU processes ---
# Default: one stray compute process occupying VRAM (matching typical debugging scenario)
DEFAULT_NVIDIA_PIDS="45231:python_train:2048"
# Default: one NPU process occupying Ascend chip
DEFAULT_NPU_PIDS="78901:AICore:4096"

# --- CASE 1: Handle nvidia-smi READ commands ---
if [[ "$0" == *"nvidia-smi"* ]]; then

    # Subcommand 1a: nvidia-smi pmon -s um -c 1
    # Process monitor: show active PIDs, type (C/G), and memory consumption
    if [[ "$ARGS" == *"pmon"* && "$ARGS" == *"-s um"* ]]; then
        PIDS=${MOCK_NVIDIA_PIDS:-$DEFAULT_NVIDIA_PIDS}
        echo "# gpu     pid  type    sm   mem   enc   dec   command"
        echo "# Idx       #    C/G     %     %     %     %   name"
        IFS=',' read -ra PID_LIST <<< "$PIDS"
        idx=0
        for entry in "${PID_LIST[@]}"; do
            IFS=':' read -r pid proc_name used_mem <<< "$entry"
            echo "  $idx   $pid    C     -    $((used_mem / 100))     -     -   $proc_name"
            idx=$((idx + 1))
        done
        exit 0
    fi

    # Subcommand 1b: nvidia-smi --query-compute-apps=pid,process_name,used_memory --format=csv
    # Query compute applications with PID, process name, and used memory
    if [[ "$ARGS" == *"--query-compute-apps"* && "$ARGS" == *"--format=csv"* ]]; then
        PIDS=${MOCK_NVIDIA_PIDS:-$DEFAULT_NVIDIA_PIDS}
        echo "pid, process_name, used_memory"
        IFS=',' read -ra PID_LIST <<< "$PIDS"
        for entry in "${PID_LIST[@]}"; do
            IFS=':' read -r pid proc_name used_mem <<< "$entry"
            echo "$pid, $proc_name, ${used_mem} MiB"
        done
        exit 0
    fi

    # Fallback for generic nvidia-smi calls (including --gpu-reset)
    exec /opt/mock-xpu-backup/nvidia-smi "$@"
fi

# --- CASE 2: Handle npu-smi READ commands ---
if [[ "$0" == *"npu-smi"* ]]; then

    # Subcommand 2a: npu-smi info -t proc-mem
    # Query process memory allocation across all available chips
    if [[ "$ARGS" == *"info"* && "$ARGS" == *"-t proc-mem"* ]]; then
        PIDS=${MOCK_NPU_PIDS:-$DEFAULT_NPU_PIDS}
        echo "Device ID   Chip ID   PID       Type        Used Memory"
        echo "----------------------------------------------------------"
        IFS=',' read -ra PID_LIST <<< "$PIDS"
        dev_id=0
        chip_id=0
        for entry in "${PID_LIST[@]}"; do
            IFS=':' read -r pid ptype used_mem <<< "$entry"
            echo "$dev_id          $chip_id       $pid    $ptype     ${used_mem} MB"
            dev_id=$((dev_id + 1))
        done
        exit 0
    fi

    # Subcommand 2b: npu-smi info -t procs -i <device_id> -c <chip_id>
    # Query running contexts on a specific device and chip
    if [[ "$ARGS" == *"info"* && "$ARGS" == *"-t procs"* ]]; then
        device_id=$(echo "$ARGS" | grep -oP '(?<=-i\s)\d+' | head -1)
        chip_id=$(echo "$ARGS" | grep -oP '(?<=-c\s)\d+' | head -1)
        device_id=${device_id:-0}
        chip_id=${chip_id:-0}

        PIDS=${MOCK_NPU_PIDS:-$DEFAULT_NPU_PIDS}
        echo "Device $device_id, Chip $chip_id - Running Processes:"
        echo "PID       Type        Used Memory"
        echo "----------------------------------"
        IFS=',' read -ra PID_LIST <<< "$PIDS"
        for entry in "${PID_LIST[@]}"; do
            IFS=':' read -r pid ptype used_mem <<< "$entry"
            echo "$pid    $ptype     ${used_mem} MB"
        done
        exit 0
    fi

    # Fallback for generic npu-smi calls (including set -t reset)
    exec /opt/mock-xpu-backup/npu-smi "$@"
fi

# --- CASE 3: Handle crictl inspectp commands ---
# Used to trace a host PID back to its container ID
if [[ "$0" == *"crictl"* ]]; then
    if [[ "$ARGS" == *"inspectp"* ]]; then
        container_id=${MOCK_CONTAINER_ID:-"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"}
        echo "{"
        echo "  \"id\": \"$container_id\","
        echo "  \"pid\": 45231,"
        echo "  \"info\": {"
        echo "    \"runtimeSpec\": {"
        echo "      \"process\": {"
        echo "        \"args\": [\"python\", \"train.py\"],"
        echo "        \"cwd\": \"/workspace\""
        echo "      }"
        echo "    },"
        echo "    \"config\": {"
        echo "      \"name\": \"stray-training-pod\""
        echo "    }"
        echo "  }"
        echo "}"
        exit 0
    fi

    # Fallback for other crictl subcommands (ps, logs, etc.)
    exec /opt/mock-xpu-backup/crictl "$@"
fi

# Fallback for unknown commands
echo "Mock XPU: Unknown command: $0 $ARGS" >&2
exit 1