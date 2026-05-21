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

package device

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// MockDeviceConfig contains common mock configuration for all device types
type MockDeviceConfig struct {
	// SkipHealthCheck skips health check in mock mode
	SkipHealthCheck bool `yaml:"mockModeSkipHealthCheck"`
	// DefaultDeviceNum is the default number of devices
	DefaultDeviceNum int32 `yaml:"defaultDeviceNum"`
	// DefaultMemory is the default memory per device (MB)
	DefaultMemory int32 `yaml:"defaultMemory"`
	// DefaultCores is the default cores per device (percentage 0-100)
	DefaultCores int32 `yaml:"defaultCores"`
	// DynamicConfigFile path to JSON file for dynamic configuration
	DynamicConfigFile string `yaml:"dynamicConfigFile"`
}

// DynamicConfigManagerAlias is an alias for backward compatibility
// Device implementations should import dynamic package directly

// CreateDynamicManager creates a dynamic config manager if enabled
func CreateDynamicManager(dynamicConfigFile string) interface{} {
	if dynamicConfigFile == "" {
		return nil
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Warning("NODE_NAME env not set, dynamic adjustment will be disabled")
		return nil
	}

	// Create in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Warningf("Failed to create in-cluster config: %v, dynamic adjustment will be disabled", err)
		return nil
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Warningf("Failed to create clientset: %v, dynamic adjustment will be disabled", err)
		return nil
	}

	return struct {
		Clientset  kubernetes.Interface
		NodeName   string
		ConfigFile string
	}{
		Clientset:  clientset,
		NodeName:   nodeName,
		ConfigFile: dynamicConfigFile,
	}
}

// GenerateMockResources generates mock resources based on configuration
func GenerateMockResources(deviceCount, memoryPerDevice, coresPerDevice int,
	memoryResourceName, coreResourceName string) map[string]int {

	resourceMap := map[string]int{}

	if memoryResourceName != "" {
		resourceMap[memoryResourceName] = memoryPerDevice * deviceCount
	}

	if coreResourceName != "" {
		resourceMap[coreResourceName] = coresPerDevice * deviceCount
	}

	klog.Infof("mock mode: generating %d devices with %d MB memory, %d cores each",
		deviceCount, memoryPerDevice, coresPerDevice)

	return resourceMap
}
