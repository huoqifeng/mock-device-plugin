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

package hygon

import (
	"errors"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type HygonConfig struct {
	ResourceCountName  string `yaml:"resourceCountName"`
	ResourceMemoryName string `yaml:"resourceMemoryName"`
	ResourceCoreName   string `yaml:"resourceCoreName"`
	MemoryFactor       int32  `yaml:"memoryFactor"`
	// Mock mode configuration
	MockModeSkipHealthCheck bool   `yaml:"mockModeSkipHealthCheck"`
	DefaultDeviceNum        int32  `yaml:"defaultDeviceNum"`
	DefaultMemory           int32  `yaml:"defaultMemory"`
	DynamicConfigFile       string `yaml:"dynamicConfigFile"`
}

type DCUDevices struct {
	config         HygonConfig
	dynamicManager interface{}
}

var (
	HygonResourceCount  string
	HygonResourceMemory string
	HygonResourceCores  string
	MemoryFactor        int32
)

const (
	RegisterAnnos      = "hami.io/node-dcu-register"
	HygonDCUDevice     = "DCU"
	HygonDCUCommonWord = "DCU"
)

func InitDCUDevice(config HygonConfig) *DCUDevices {
	HygonResourceCount = config.ResourceCountName
	HygonResourceMemory = config.ResourceMemoryName
	HygonResourceCores = config.ResourceCoreName
	MemoryFactor = config.MemoryFactor

	return &DCUDevices{
		config: config,
	}
}

func (dev *DCUDevices) CommonWord() string {
	return HygonDCUCommonWord
}

func (dev *DCUDevices) GetNodeDevices(n *corev1.Node) ([]*device.DeviceInfo, error) {
	devEncoded, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return []*device.DeviceInfo{}, errors.New("annos not found " + RegisterAnnos)
	}
	nodedevices, err := device.DecodeNodeDevices(devEncoded)
	if err != nil {
		klog.ErrorS(err, "failed to decode node devices", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, err
	}
	for idx := range nodedevices {
		nodedevices[idx].DeviceVendor = HygonDCUCommonWord
	}
	if len(nodedevices) == 0 {
		klog.InfoS("no gpu device found", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, errors.New("no gpu found on node")
	}
	devDecoded := device.EncodeNodeDevices(nodedevices)
	klog.V(5).InfoS("nodes device information", "node", n.Name, "nodedevices", devDecoded)
	return nodedevices, nil
}

func (dev *DCUDevices) GetResource(n *corev1.Node) map[string]int {
	memoryResourceName := device.GetResourceName(HygonResourceMemory)
	coreResourceName := device.GetResourceName(HygonResourceCores)
	resourceMap := map[string]int{
		memoryResourceName: 0,
	}

	// Skip health check in mock mode
	if !dev.config.MockModeSkipHealthCheck {
		if !device.CheckHealthy(n, HygonResourceCount) {
			klog.Infof("device %s is unhealthy on this node", dev.CommonWord())
			return resourceMap
		}
	} else {
		klog.V(5).Infof("mock mode enabled, skipping health check for device %s", dev.CommonWord())
	}

	devs, err := dev.GetNodeDevices(n)
	if err != nil {
		// In mock mode, generate default devices if annotation not found
		if dev.config.MockModeSkipHealthCheck && dev.config.DefaultDeviceNum > 0 {
			deviceCount := int(dev.config.DefaultDeviceNum)
			memoryPerDevice := int(dev.config.DefaultMemory)
			coresPerDevice := 30 // Default cores

			klog.Infof("mock mode: generating %d DCU devices with %d MB memory each",
				deviceCount, memoryPerDevice)

			resourceMap[memoryResourceName] = memoryPerDevice * deviceCount
			if coreResourceName != "" {
				resourceMap[coreResourceName] = coresPerDevice * deviceCount
			}
		} else {
			klog.Infof("no device %s on this node", dev.CommonWord())
			return resourceMap
		}
	} else {
		for _, val := range devs {
			resourceMap[memoryResourceName] += int(val.Devmem)
		}
	}
	if MemoryFactor > 1 {
		rawMemory := resourceMap[memoryResourceName]
		resourceMap[memoryResourceName] /= int(MemoryFactor)
		klog.InfoS("Update memory", "raw", rawMemory, "after", resourceMap[memoryResourceName], "factor", MemoryFactor)
	}
	klog.InfoS("Add resources", memoryResourceName, resourceMap[memoryResourceName])
	return resourceMap
}

func (dev *DCUDevices) RunManager() {
	lmock := mock.NewMockLister(device.GetVendorName(HygonResourceMemory))
	device.Register(lmock, dev)
	mockmanager := dpm.NewManager(lmock)
	klog.Infof("Running mocking dp: %s", dev.CommonWord())
	mockmanager.Run()
}
