# Mock Mode Implementation Guide for All Device Types

## Overview

This guide explains how to enable mock mode and dynamic adjustment for all supported device types in the enhanced mock-device-plugin.

## Supported Device Types

1. ✅ **NVIDIA GPU** - Fully implemented
2. 🔄 **Hygon DCU** - Partially implemented  
3. ⏳ **AMD GPU** - Needs implementation
4. ⏳ **Ascend NPU** (Huawei) - Needs implementation
5. ⏳ **AWS Neuron** - Needs implementation
6. ⏳ **Cambricon MLU** - Needs implementation
7. ⏳ **Enflame GPU** - Needs implementation
8. ⏳ **Iluvatar GPU** - Needs implementation
9. ⏳ **Kunlun XPU** - Needs implementation
10. ⏳ **MetaX GPU** - Needs implementation
11. ⏳ **Mthreads GPU** - Needs implementation

## Implementation Pattern

For each device type, you need to modify:

### 1. Config Struct

Add these fields to the device-specific config struct:

```go
type DeviceConfig struct {
    // ... existing fields ...
    
    // Mock mode configuration
    MockModeSkipHealthCheck bool   `yaml:"mockModeSkipHealthCheck"`
    DefaultDeviceNum        int32  `yaml:"defaultDeviceNum"`
    DefaultMemory           int32  `yaml:"defaultMemory"`
    DefaultCores            int32  `yaml:"defaultCores"`
    DynamicConfigFile       string `yaml:"dynamicConfigFile"`
}
```

### 2. Device Struct

Store the config in the device struct:

```go
type DeviceImplementation struct {
    config         DeviceConfig
    dynamicManager *dynamic.DynamicConfigManager  // optional
}
```

### 3. Init Function

Update initialization to store config:

```go
func InitDevice(config DeviceConfig) *DeviceImplementation {
    return &DeviceImplementation{
        config: config,
    }
}
```

### 4. GetResource Function

Modify to support mock mode:

```go
func (dev *DeviceImplementation) GetResource(n *corev1.Node) map[string]int {
    resourceMap := map[string]int{
        memoryResourceName: 0,
        // ... other resources
    }
    
    // Skip health check in mock mode
    if !dev.config.MockModeSkipHealthCheck {
        if !device.CheckHealthy(n, ResourceCountName) {
            klog.Infof("device %s is unhealthy", dev.CommonWord())
            return resourceMap
        }
    } else {
        klog.V(5).Infof("mock mode enabled, skipping health check")
    }
    
    devs, err := dev.GetNodeDevices(n)
    if err != nil {
        // In mock mode, generate default devices
        if dev.config.MockModeSkipHealthCheck && dev.config.DefaultDeviceNum > 0 {
            deviceCount := int(dev.config.DefaultDeviceNum)
            memoryPerDevice := int(dev.config.DefaultMemory)
            coresPerDevice := int(dev.config.DefaultCores)
            
            klog.Infof("mock mode: generating %d devices", deviceCount)
            
            resourceMap[memoryResourceName] = memoryPerDevice * deviceCount
            resourceMap[coreResourceName] = coresPerDevice * deviceCount
        } else {
            klog.Infof("no device on this node")
            return resourceMap
        }
    } else {
        for _, val := range devs {
            resourceMap[memoryResourceName] += int(val.Devmem)
            resourceMap[coreResourceName] += int(val.Devcore)
        }
    }
    
    return resourceMap
}
```

## Configuration Example

### Complete ConfigMap for All Devices

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mock-device-config
  namespace: kube-system
data:
  device-config.yaml: |
    nvidia:
      resourceCountName: nvidia.com/gpu
      resourceMemoryName: nvidia.com/gpumem
      resourceCoreName: nvidia.com/gpucores
      defaultDeviceNum: 4
      defaultMemory: 8192
      defaultCores: 30
      mockModeSkipHealthCheck: true
      dynamicConfigFile: /etc/mock-device/dynamic-config.json
    
    hygon:
      resourceCountName: hygon.com/dcu
      resourceMemoryName: hygon.com/dcumem
      defaultDeviceNum: 2
      defaultMemory: 16384
      mockModeSkipHealthCheck: true
    
    ascend:
      resourceCountName: huawei.com/Ascend910
      resourceMemoryName: huawei.com/Ascend910-memory
      defaultDeviceNum: 4
      defaultMemory: 32768
      mockModeSkipHealthCheck: true
    
    # ... other devices
```

## Device-Specific Resource Names

| Device Type | Count Resource | Memory Resource | Core Resource |
|------------|----------------|-----------------|---------------|
| NVIDIA | nvidia.com/gpu | nvidia.com/gpumem | nvidia.com/gpucores |
| Hygon DCU | hygon.com/dcu | hygon.com/dcumem | - |
| AMD GPU | amd.com/gpu | amd.com/gpumem | - |
| Ascend NPU | huawei.com/Ascend910 | huawei.com/Ascend910-memory | - |
| AWS Neuron | aws.amazon.com/neuron | - | - |
| Cambricon MLU | cambricon.com/mlu | - | - |

## Testing Mock Mode

### 1. Build Image
```bash
docker build -f Dockerfile.mock-device-plugin -t mock-device-plugin:enhanced .
```

### 2. Deploy
```bash
kubectl apply -f mock-device-config.yaml
kubectl apply -f mock-device-daemonset.yaml
```

### 3. Verify
```bash
# Check for NVIDIA
kubectl get nodes -o custom-columns=NAME:.metadata.name,GPU:.status.allocatable.nvidia\\.com/gpumem

# Check for Hygon DCU
kubectl get nodes -o custom-columns=NAME:.metadata.name,DCU:.status.allocatable.hygon\\.com/dcumem

# Check for Ascend NPU
kubectl get nodes -o custom-columns=NAME:.metadata.name,NPU:.status.allocatable.huawei\\.com/Ascend910-memory
```

## Status

- ✅ **NVIDIA**: Complete implementation with dynamic adjustment
- 🔄 **Hygon DCU**: Config fields added, needs GetResource() completion
- ⏳ **Others**: Needs full implementation following the pattern above

## Next Steps

1. Complete Hygon DCU implementation
2. Apply same pattern to AMD GPU
3. Apply to Ascend NPU
4. Continue with remaining device types
5. Test each device type individually
6. Update documentation with specific resource names