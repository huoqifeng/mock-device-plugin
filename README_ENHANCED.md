# Enhanced Mock Device Plugin for HAMi

This is an enhanced version of the mock-device-plugin with the following additional features:

## New Features

### 1. Mock Mode Health Check Skip

In mock mode, the plugin now skips the health check that was causing issues in pure mock deployments. This is controlled by the `mockModeSkipHealthCheck` configuration option.

**Configuration:**
```yaml
nvidia:
  resourceCountName: nvidia.com/gpu
  resourceMemoryName: nvidia.com/gpumem
  resourceCoreName: nvidia.com/gpucores
  mockModeSkipHealthCheck: true  # Enable mock mode
```

### 2. Dynamic Device Configuration via JSON File

Instead of using a ConfigMap, you can now use a local JSON file to configure device parameters dynamically. This allows for runtime adjustments without restarting the plugin.

**Configuration:**
```yaml
nvidia:
  dynamicConfigFile: /etc/mock-device/dynamic-config.json
```

**JSON File Format:**
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

**Parameters:**
- `deviceCount`: Number of virtual GPU devices to advertise
- `memoryPerDevice`: Memory (in MB) per device
- `coresPerDevice`: Cores (percentage 0-100) per device
- `autoAdjust`: Enable automatic adjustment based on pod consumption
- `minDevices`: Minimum devices when auto-adjusting
- `maxDevices`: Maximum devices when auto-adjusting
- `adjustmentInterval`: How often to adjust (seconds)

### 3. Dynamic Device Adjustment Based on Pod Consumption

When `autoAdjust` is enabled, the plugin will automatically adjust the number of virtual devices based on the total GPU resources requested by running pods on the node.

**How it works:**
1. The plugin periodically checks all running pods on the node
2. It calculates the total GPU resources requested (`nvidia.com/gpu`)
3. It adjusts the device count to match the demand
4. Device count is bounded by `minDevices` and `maxDevices`

**Example Use Case:**
```json
{
  "deviceCount": 4,
  "autoAdjust": true,
  "minDevices": 2,
  "maxDevices": 10,
  "adjustmentInterval": 30
}
```

This configuration will:
- Start with 4 devices
- Every 30 seconds, check pod resource requests
- Adjust device count between 2-10 based on demand

## Deployment

### Option 1: Using ConfigMap (Traditional)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: hami-scheduler-device
  namespace: kube-system
data:
  device-config.yaml: |
    nvidia:
      resourceCountName: nvidia.com/gpu
      resourceMemoryName: nvidia.com/gpumem
      resourceCoreName: nvidia.com/gpucores
      mockModeSkipHealthCheck: true
```

### Option 2: Using JSON File (Recommended for Dynamic Config)

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: mock-gpu-device-plugin
  namespace: kube-system
spec:
  template:
    spec:
      containers:
      - name: mock-plugin
        image: mock-device-plugin:latest
        command:
          - ./device-plugin
          - -v=5
          - --device-config-file=/etc/mock-device/device-config.yaml
        volumeMounts:
        - name: device-config
          mountPath: /etc/mock-device
        - name: dynamic-config
          mountPath: /etc/mock-device/dynamic-config.json
          subPath: dynamic-config.json
      volumes:
      - name: device-config
        configMap:
          name: device-config-mock
      - name: dynamic-config
        configMap:
          name: dynamic-device-config
```

## Building

```bash
# Build the enhanced version
docker build -t mock-device-plugin:enhanced .

# Or using the custom Dockerfile
docker build -f Dockerfile.mock-device-plugin -t mock-device-plugin:latest .
```

## Testing

1. **Deploy with Mock Mode:**
   ```bash
   kubectl apply -f k8s-mock-rbac.yaml
   kubectl apply -f device-config-mock.yaml
   kubectl apply -f dynamic-device-config.yaml
   kubectl apply -f k8s-mock-plugin.yaml
   ```

2. **Verify GPU Resources:**
   ```bash
   kubectl get nodes -o custom-columns=NAME:.metadata.name,GPU:.status.allocatable.nvidia\\.com/gpu
   ```

3. **Test Dynamic Adjustment:**
   - Create a pod requesting GPU resources
   - Watch the device count adjust automatically
   - Check logs: `kubectl logs -n kube-system <pod-name>`

## Configuration Examples

### Minimal Configuration (Static 4 GPUs)
```yaml
nvidia:
  resourceCountName: nvidia.com/gpu
  mockModeSkipHealthCheck: true
  defaultGPUNum: 4
```

### Dynamic Configuration (Auto-adjusting)
```yaml
nvidia:
  resourceCountName: nvidia.com/gpu
  mockModeSkipHealthCheck: true
  dynamicConfigFile: /etc/mock-device/dynamic-config.json
```

With `/etc/mock-device/dynamic-config.json`:
```json
{
  "deviceCount": 4,
  "autoAdjust": true,
  "minDevices": 1,
  "maxDevices": 8,
  "adjustmentInterval": 60
}
```

## Troubleshooting

### Device Not Showing Up
- Check `mockModeSkipHealthCheck` is set to `true`
- Verify ConfigMap is mounted correctly
- Check plugin logs for errors

### Dynamic Adjustment Not Working
- Ensure `autoAdjust` is `true` in JSON config
- Verify RBAC permissions for listing pods
- Check adjustment interval is reasonable

## Differences from Upstream

1. Added `mockModeSkipHealthCheck` field to NvidiaConfig
2. Added `dynamicConfigFile` field to NvidiaConfig  
3. New `dynamic` package for configuration management
4. Auto-adjustment based on pod consumption
5. JSON-based dynamic configuration

## Maintainer

Original: limengxuan@4paradigm.com  
Enhanced by: huoqifeng

## License

Apache License 2.0