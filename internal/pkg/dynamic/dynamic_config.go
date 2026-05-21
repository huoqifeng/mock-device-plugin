/*
Copyright 2025 The HAMi Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
)

// DynamicDeviceConfig represents the dynamic device configuration
type DynamicDeviceConfig struct {
	// DeviceCount is the number of virtual devices to advertise
	DeviceCount int `json:"deviceCount"`
	// MemoryPerDevice is the amount of memory (in MB) per device
	MemoryPerDevice int `json:"memoryPerDevice"`
	// CoresPerDevice is the number of cores per device (percentage, 0-100)
	CoresPerDevice int `json:"coresPerDevice"`
	// AutoAdjust enables automatic adjustment based on pod consumption
	AutoAdjust bool `json:"autoAdjust"`
	// MinDevices is the minimum number of devices when auto-adjusting
	MinDevices int `json:"minDevices"`
	// MaxDevices is the maximum number of devices when auto-adjusting
	MaxDevices int `json:"maxDevices"`
	// AdjustmentInterval is how often to adjust devices (in seconds)
	AdjustmentInterval int `json:"adjustmentInterval"`
}

// DynamicConfigManager manages dynamic device configuration
type DynamicConfigManager struct {
	configFile string
	config     *DynamicDeviceConfig
	clientset  kubernetes.Interface
	nodeName   string
	mutex      sync.RWMutex
	stopChan   chan struct{}
}

// NewDynamicConfigManager creates a new dynamic config manager
func NewDynamicConfigManager(configFile string, clientset kubernetes.Interface, nodeName string) *DynamicConfigManager {
	return &DynamicConfigManager{
		configFile: configFile,
		clientset:  clientset,
		nodeName:   nodeName,
		stopChan:   make(chan struct{}),
	}
}

// LoadConfig loads the configuration from JSON file
func (m *DynamicConfigManager) LoadConfig() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	data, err := os.ReadFile(m.configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config if file doesn't exist
			klog.Infof("Config file %s not found, creating default config", m.configFile)
			return m.createDefaultConfig()
		}
		return fmt.Errorf("failed to read config file: %v", err)
	}

	var config DynamicDeviceConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	m.config = &config
	klog.Infof("Loaded dynamic config: %+v", config)
	return nil
}

// createDefaultConfig creates a default configuration file
func (m *DynamicConfigManager) createDefaultConfig() error {
	defaultConfig := &DynamicDeviceConfig{
		DeviceCount:        4,
		MemoryPerDevice:    8192,
		CoresPerDevice:     30,
		AutoAdjust:         false,
		MinDevices:         1,
		MaxDevices:         8,
		AdjustmentInterval: 60,
	}

	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %v", err)
	}

	if err := os.WriteFile(m.configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write default config: %v", err)
	}

	m.config = defaultConfig
	return nil
}

// GetConfig returns the current configuration
func (m *DynamicConfigManager) GetConfig() *DynamicDeviceConfig {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.config
}

// UpdateConfig updates the configuration and saves to file
func (m *DynamicConfigManager) UpdateConfig(newConfig *DynamicDeviceConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	data, err := json.MarshalIndent(newConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(m.configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	m.config = newConfig
	klog.Infof("Updated dynamic config: %+v", newConfig)
	return nil
}

// StartAutoAdjustment starts the automatic device adjustment based on pod consumption
func (m *DynamicConfigManager) StartAutoAdjustment() {
	if m.config == nil || !m.config.AutoAdjust {
		klog.Info("Auto-adjustment is disabled")
		return
	}

	interval := time.Duration(m.config.AdjustmentInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	klog.Infof("Starting auto-adjustment with interval %v", interval)

	for {
		select {
		case <-ticker.C:
			if err := m.adjustDevices(); err != nil {
				klog.Errorf("Failed to adjust devices: %v", err)
			}
		case <-m.stopChan:
			klog.Info("Stopping auto-adjustment")
			return
		}
	}
}

// adjustDevices adjusts the number of devices based on current pod consumption
func (m *DynamicConfigManager) adjustDevices() error {
	if m.clientset == nil {
		return fmt.Errorf("clientset is nil")
	}

	// Get all pods on this node
	pods, err := m.clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase=Running", m.nodeName),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	// Calculate total GPU resources requested
	totalGPURequested := int64(0)
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			if gpuRequest, ok := container.Resources.Requests[corev1.ResourceName("nvidia.com/gpu")]; ok {
				totalGPURequested += gpuRequest.Value()
			}
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Calculate optimal device count
	// Simple algorithm: each device can handle a certain number of pods
	// Adjust this based on your needs
	currentDevices := m.config.DeviceCount
	optimalDevices := int(totalGPURequested)

	if optimalDevices < m.config.MinDevices {
		optimalDevices = m.config.MinDevices
	}
	if optimalDevices > m.config.MaxDevices {
		optimalDevices = m.config.MaxDevices
	}

	if optimalDevices != currentDevices {
		klog.Infof("Adjusting devices from %d to %d (requested: %d)", currentDevices, optimalDevices, totalGPURequested)
		m.config.DeviceCount = optimalDevices

		// Save updated config
		data, err := json.MarshalIndent(m.config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config: %v", err)
		}
		if err := os.WriteFile(m.configFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write config: %v", err)
		}
	}

	return nil
}

// Stop stops the auto-adjustment routine
func (m *DynamicConfigManager) Stop() {
	close(m.stopChan)
}

// GenerateDeviceInfo generates device info based on current dynamic config
func (m *DynamicConfigManager) GenerateDeviceInfo() []*device.DeviceInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.config == nil {
		return nil
	}

	devices := make([]*device.DeviceInfo, m.config.DeviceCount)
	for i := 0; i < m.config.DeviceCount; i++ {
		devices[i] = &device.DeviceInfo{
			ID:      fmt.Sprintf("GPU-%d", i),
			Index:   uint(i),
			Count:   1,
			Devmem:  int32(m.config.MemoryPerDevice),
			Devcore: int32(m.config.CoresPerDevice),
			Type:    "NVIDIA",
			Numa:    0,
			Mode:    "mock",
			Health:  true,
		}
	}

	return devices
}
