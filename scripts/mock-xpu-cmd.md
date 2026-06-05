# mock-xpu-cmd.sh — Usage & Reference

Mock script for XPU device debugging commands (NVIDIA GPU & Huawei Ascend NPU).
Only **READ-only** commands are intercepted; WRITE commands (`--gpu-reset`, `set -t reset`, `kill`) fall through to the real binaries.

## Installation

### Step 1: Backup original binaries

> **Important**: On many workers `/usr/local/bin/crictl`, `/usr/local/bin/nvidia-smi`, etc. are **real binaries** (not symlinks).
> `ln -sf` would **permanently overwrite** them. Always backup first.

```bash
mkdir -p /opt/mock-xpu-backup
for cmd in nvidia-smi npu-smi crictl; do
    if [ -f "/usr/local/bin/$cmd" ] && [ ! -L "/usr/local/bin/$cmd" ]; then
        cp "/usr/local/bin/$cmd" "/opt/mock-xpu-backup/$cmd"
        echo "Backed up /usr/local/bin/$cmd -> /opt/mock-xpu-backup/$cmd"
    fi
done
```

### Step 2: Install the mock script

```bash
cp mock-xpu-cmd.sh /opt/mock-xpu-cmd.sh
chmod +x /opt/mock-xpu-cmd.sh
```

### Step 3: Create symlinks (overwrites backed-up binaries)

```bash
ln -sf /opt/mock-xpu-cmd.sh /usr/local/bin/nvidia-smi
ln -sf /opt/mock-xpu-cmd.sh /usr/local/bin/npu-smi
ln -sf /opt/mock-xpu-cmd.sh /usr/local/bin/crictl
```

### Uninstall (restore originals)

```bash
# Restore backed-up binaries
for cmd in nvidia-smi npu-smi crictl; do
    if [ -f "/opt/mock-xpu-backup/$cmd" ]; then
        cp "/opt/mock-xpu-backup/$cmd" "/usr/local/bin/$cmd"
        echo "Restored /usr/local/bin/$cmd from backup"
    fi
done

# Clean up
rm -rf /opt/mock-xpu-cmd.sh /opt/mock-xpu-backup /etc/mock-xpu.env
```

## Configuration

Create `/etc/mock-xpu.env` to override defaults:

```bash
# GPU processes: pid:process_name:used_memory_mib (comma-separated for multiple)
MOCK_NVIDIA_PIDS="45231:python_train:2048,67890:jupyter_notebook:512"

# NPU processes: pid:type:used_memory_mb (comma-separated for multiple)
MOCK_NPU_PIDS="78901:AICore:4096,78902:AICore:2048"

# Container ID returned by crictl inspectp
MOCK_CONTAINER_ID="a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"
```

If the env file is absent, built-in defaults are used:

| Variable            | Default                                  |
|---------------------|------------------------------------------|
| `MOCK_NVIDIA_PIDS`  | `45231:python_train:2048`                |
| `MOCK_NPU_PIDS`     | `78901:AICore:4096`                      |
| `MOCK_CONTAINER_ID` | `a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6`      |

## Supported Commands

### Phase 1 — NVIDIA GPU

#### 1a. Process monitor

```bash
nvidia-smi pmon -s um -c 1
```

Output:

```
# gpu     pid  type    sm   mem   enc   dec   command
# Idx       #    C/G     %     %     %     %   name
  0   45231    C     -    20     -     -   python_train
```

#### 1b. Query compute applications

```bash
nvidia-smi --query-compute-apps=pid,process_name,used_memory --format=csv
```

Output:

```
pid, process_name, used_memory
45231, python_train, 2048 MiB
```

### Phase 2 — Huawei Ascend NPU

#### 2a. Process memory across all chips

```bash
npu-smi info -t proc-mem
```

Output:

```
Device ID   Chip ID   PID       Type        Used Memory
----------------------------------------------------------
0          0       78901    AICore     4096 MB
```

#### 2b. Processes on a specific chip

```bash
npu-smi info -t procs -i 0 -c 0
```

Output:

```
Device 0, Chip 0 - Running Processes:
PID       Type        Used Memory
----------------------------------
78901    AICore     4096 MB
```

### Phase 3 — Container tracing

```bash
crictl inspectp --id <container_id>
```

Returns JSON with container metadata (id, pid, process args, working dir, pod name).

## Write Commands (Not Mocked)

The following commands are **not intercepted** and execute against the real binaries:

| Command                        | Reason          |
|--------------------------------|-----------------|
| `nvidia-smi --gpu-reset -i N`  | Hardware write  |
| `npu-smi set -t reset -i N -c N` | Hardware write |
| `kill -9 <PID>`                | Process write   |

## Design Notes

- The script uses `$0` (symlink name) to determine which tool is being called.
- Unmatched subcommands fall through to the real binary via `exec`.
- All mock output formats aim to match real tool output for drop-in compatibility.