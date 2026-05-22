# Volcano GPU Sharing Support

This document describes the Volcano GPU Sharing support added to mock-device-plugin.

## Overview

The mock-device-plugin now supports Volcano's GPU Sharing feature by exposing GPU resources in Volcano's expected format.

## Changes Made

### 1. Added Volcano Resource Constants

```go
// mock-device-plugin/internal/pkg/api/device/nvidia/device.go

const (
    // HAMi format (existing)
    RegisterAnnos        = "hami.io/node-nvidia-register"
    RegisterGPUPairScore = "hami.io/node-nvidia-score"
    
    // Volcano GPU Sharing format (new)
    VolcanoGPUResource = "volcano.sh/gpu-memory"
    VolcanoGPUNumber   = "volcano.sh/gpu-number"
    VolcanoGPUIndex    = "volcano.sh/gpu-index"
)
```

### 2. Modified GetResource Function

The `GetResource` function now returns both HAMi and Volcano resource formats:

```go
resourceMap := map[string]int{
    memoryResourceName:   0,  // HAMi format
    coreResourceName:     0,  // HAMi format
    memoryPercentageName: 0,  // HAMi format
    VolcanoGPUResource:   0,  // Volcano format (new)
    VolcanoGPUNumber:     0,  // Volcano format (new)
}
```

### 3. Resource Mapping

| HAMi Resource | Volcano Resource | Description |
|--------------|------------------|-------------|
| `hami.io/gpu-memory` | `volcano.sh/gpu-memory` | Total GPU memory in MB |
| N/A | `volcano.sh/gpu-number` | Number of GPU devices |

## How It Works

1. **Device Registration**: When mock-device-plugin starts, it registers GPU devices with both HAMi and Volcano resource names.

2. **Node Capacity**: The resources are exposed in `node.Status.Capacity`:
   ```yaml
   status:
     capacity:
       nvidia.com/gpu: "4"
       volcano.sh/gpu-memory: "65536"  # Total memory in MB
       volcano.sh/gpu-number: "4"      # Number of GPUs
   ```

3. **Volcano Scheduler**: Volcano scheduler reads `volcano.sh/gpu-memory` and `volcano.sh/gpu-number` from node capacity to enable GPU sharing.

## Usage

### With HAMi

The existing HAMi functionality remains unchanged:
```yaml
resources:
  limits:
    nvidia.com/gpu: 1
    hami.io/gpu-memory: 16384
```

### With Volcano

Now you can use Volcano's GPU sharing:
```yaml
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: gpu-job
spec:
  schedulerName: volcano
  tasks:
    - replicas: 1
      name: "gpu-task"
      template:
        spec:
          containers:
            - name: gpu-container
              image: nvidia/cuda:11.0-base
              resources:
                limits:
                  nvidia.com/gpu: 1
                  volcano.sh/gpu-memory: "16384"  # Request 16GB GPU memory
```

## Volcano Scheduler Configuration

Enable GPU sharing in Volcano scheduler config:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: volcano-scheduler-configmap
  namespace: volcano-system
data:
  volcano-scheduler.conf: |
    actions: "enqueue, allocate, backfill"
    tiers:
    - plugins:
      - name: priority
      - name: gang
      - name: conformance
    - plugins:
      - name: drf
      - name: predicates
        enablePredicate:
          - GPUSharingPredicate  # Enable GPU sharing
      - name: proportion
      - name: nodeorder
```

## Verification

### Check Node Resources

```bash
kubectl get node <node-name> -o jsonpath='{.status.capacity}' | jq
```

Expected output:
```json
{
  "cpu": "8",
  "memory": "32Gi",
  "nvidia.com/gpu": "4",
  "volcano.sh/gpu-memory": "65536",
  "volcano.sh/gpu-number": "4"
}
```

### Check GPU Allocation

```bash
# Check CPU Manager state
docker exec <worker-node> cat /var/lib/kubelet/cpu_manager_state

# Check Memory Manager state  
docker exec <worker-node> cat /var/lib/kubelet/memory_manager_state
```

## Compatibility

- ✅ HAMi: Full backward compatibility maintained
- ✅ Volcano gpu-numa branch: Full support for GPU sharing
- ✅ Standard Kubernetes: Works with standard device plugin interface

## References

- [Volcano GPU Sharing Design](https://github.com/volcano-sh/volcano/blob/master/docs/design/device-sharing.md)
- [Volcano GPU Sharing User Guide](https://github.com/volcano-sh/volcano/blob/master/docs/user-guide/how_to_use_gpu_sharing.md)
- [HAMi Documentation](https://github.com/HAMi/HAMi)