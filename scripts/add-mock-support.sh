#!/bin/bash

# Script to add mock mode support to all device types

DEVICES=(
    "amd"
    "ascend"
    "awsneuron"
    "cambricon"
    "enflame"
    "hygon"
    "iluvatar"
    "kunlun"
    "metax"
    "mthreads"
)

DEVICE_DIR="mock-device-plugin/internal/pkg/api/device"

for device in "${DEVICES[@]}"; do
    echo "Processing $device..."
    
    device_file="$DEVICE_DIR/$device/device.go"
    
    if [[ ! -f "$device_file" ]]; then
        echo "  Warning: $device_file not found, skipping"
        continue
    fi
    
    # Check if mock support already exists
    if grep -q "MockModeSkipHealthCheck" "$device_file"; then
        echo "  Mock support already exists in $device, skipping"
        continue
    fi
    
    echo "  Adding mock support to $device..."
    
    # This is a placeholder - actual implementation would use sed/python
    # to modify the files programmatically
    echo "  TODO: Modify $device_file to add:"
    echo "    - MockModeSkipHealthCheck field in config struct"
    echo "    - DefaultDeviceNum, DefaultMemory, DefaultCores fields"
    echo "    - Mock logic in GetResource() function"
done

echo ""
echo "Manual modifications needed for each device type:"
echo "1. Add mock config fields to DeviceConfig struct"
echo "2. Store config in Device struct"  
echo "3. Modify GetResource() to skip health check and generate mock devices"
echo "4. Update config loading in Init function"