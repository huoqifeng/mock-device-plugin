# Changes Made to Mock Device Plugin

## Overview
This fork enhances the original mock-device-plugin with three major features to better support testing and development scenarios.

## Changes

### 1. Modified Files

#### `internal/pkg/api/device/nvidia/device.go`
**Change:** Added two new configuration fields to `NvidiaConfig` struct
```go
type NvidiaConfig struct {
    // ... existing fields ...
    
    // MockModeSkipHealthCheck skips health check in mock mode
    MockModeSkipHealthCheck bool `yaml:"mockModeSkipHealthCheck"`
    // DynamicConfigFile path to JSON file for dynamic device configuration
    DynamicConfigFile string `yaml:"dynamicConfigFile"`
}
```

**Change:** Modified `GetResource()` function to support skipping health check
```go
func (dev *NvidiaGPUDevices) GetResource(n *corev1.Node) map[string]int {
    // ... code ...
    
    // Skip health check in mock mode
    if !dev.config.MockModeSkipHealthCheck {
        if !device.CheckHealthy(n, dev.config.ResourceCountName) {
            klog.Infof("device %s is unhealthy on this node", dev.CommonWord())
            return resourceMap
        }
    } else {
        klog.V(5).Infof("mock mode enabled, skipping health check for device %s", dev.CommonWord())
    }
    
    // ... rest of code ...
}
```

### 2. New Files

#### `internal/pkg/dynamic/dynamic_config.go` (New)
**Purpose:** Manages dynamic device configuration via JSON file

**Key Features:**
- `DynamicDeviceConfig` struct for device configuration
- `DynamicConfigManager` for managing configuration
- Auto-adjustment based on pod consumption
- Thread-safe configuration updates
- JSON file persistence

**Key Functions:**
- `LoadConfig()`: Load configuration from JSON file
- `UpdateConfig()`: Update and persist configuration
- `StartAutoAdjustment()`: Start automatic device adjustment goroutine
- `GenerateDeviceInfo()`: Generate device info based on current config

## Configuration

### YAML Configuration (device-config.yaml)
```yaml
nvidia:
  resourceCountName: nvidia.com/gpu
  resourceMemoryName: nvidia.com/gpumem
  resourceCoreName: nvidia.com/gpucores
  defaultMemory: 8192
  defaultCores: 30
  defaultGPUNum: 4
  mockModeSkipHealthCheck: true  # NEW: Skip health check
  dynamicConfigFile: /etc/mock-device/dynamic-config.json  # NEW: Dynamic config
```

### JSON Configuration (dynamic-config.json)
```json
{
  "deviceCount": 4,
  "memoryPerDevice": 8192,
  "coresPerDevice": 30,
  "autoAdjust": false,
  "minDevices": 1,
  "maxDevices": 8,
  "adjustmentInterval": 60
}
```

## Behavior Changes

### Before
1. Health check required nodes to already have GPU resources
2. Configuration was static, loaded only at startup
3. No way to adjust device count dynamically

### After
1. Health check can be skipped in mock mode (`mockModeSkipHealthCheck: true`)
2. Configuration can be dynamic via JSON file
3. Device count can auto-adjust based on pod consumption

## Migration Guide

### For Existing Deployments
No breaking changes. Existing configurations will continue to work.

To use new features:
1. Add `mockModeSkipHealthCheck: true` to your device config
2. Optionally add `dynamicConfigFile` for dynamic configuration
3. Create the JSON config file if using dynamic configuration

### For New Deployments
Use the new configuration files provided:
- `device-config-mock.yaml` - YAML configuration
- `dynamic-device-config.json` - JSON configuration

## Testing

### Unit Tests
TODO: Add unit tests for:
- `DynamicConfigManager` methods
- Mock mode health check skip logic
- Auto-adjustment algorithm

### Integration Tests
1. Deploy plugin with mock mode enabled
2. Verify GPU resources appear on nodes
3. Test dynamic adjustment by creating pods
4. Verify device count changes

## Known Issues

1. Auto-adjustment requires RBAC permissions to list pods
2. JSON file must be writable by the plugin process
3. Device count changes require plugin restart (limitation of device plugin API)

## Future Enhancements

1. Support for other device types (AMD, Ascend, etc.)
2. Web UI for configuration management
3. Metrics endpoint for monitoring
4. Hot reload without restart

## Commit History

1. `feat: Add mockModeSkipHealthCheck to skip health check in mock mode`
2. `feat: Add dynamicConfigFile for JSON-based configuration`
3. `feat: Implement DynamicConfigManager for dynamic device management`
4. `feat: Add auto-adjustment based on pod consumption`
5. `docs: Add README_ENHANCED.md with documentation`