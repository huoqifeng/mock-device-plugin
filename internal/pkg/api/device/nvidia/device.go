/*
Copyright 2024 The HAMi Authors.

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

package nvidia

import (
	"errors"
	"os"
	"strings"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/dynamic"
	"github.com/HAMi/mock-device-plugin/internal/pkg/mock"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	RegisterAnnos        = "hami.io/node-nvidia-register"
	RegisterGPUPairScore = "hami.io/node-nvidia-score"
	NvidiaGPUDevice      = "NVIDIA"
	NvidiaGPUCommonWord  = "GPU"
	Vendor               = "nvidia.com"
	MigMode              = "mig"
)

type LibCudaLogLevel string
type GPUCoreUtilizationPolicy string

type NvidiaConfig struct {
	// These configs are shared and can be overwritten by Nodeconfig.
	NodeDefaultConfig            `yaml:",inline"`
	ResourceCountName            string `yaml:"resourceCountName"`
	ResourceMemoryName           string `yaml:"resourceMemoryName"`
	ResourceCoreName             string `yaml:"resourceCoreName"`
	ResourceMemoryPercentageName string `yaml:"resourceMemoryPercentageName"`
	ResourcePriority             string `yaml:"resourcePriorityName"`
	OverwriteEnv                 bool   `yaml:"overwriteEnv"`
	DefaultMemory                int32  `yaml:"defaultMemory"`
	DefaultCores                 int32  `yaml:"defaultCores"`
	DefaultGPUNum                int32  `yaml:"defaultGPUNum"`
	MemoryFactor                 int32  `yaml:"memoryFactor"`
	// TODO Whether these should be removed
	DisableCoreLimit  bool                   `yaml:"disableCoreLimit"`
	MigGeometriesList []AllowedMigGeometries `yaml:"knownMigGeometries"`
	// GPUCorePolicy through webhook automatic injected to container env
	GPUCorePolicy GPUCoreUtilizationPolicy `yaml:"gpuCorePolicy"`
	// RuntimeClassName is the name of the runtime class to be added to pod.spec.runtimeClassName
	RuntimeClassName string `yaml:"runtimeClassName"`
	// MockModeSkipHealthCheck skips health check in mock mode
	MockModeSkipHealthCheck bool `yaml:"mockModeSkipHealthCheck"`
	// DynamicConfigFile path to JSON file for dynamic device configuration
	DynamicConfigFile string `yaml:"dynamicConfigFile"`
}

type AllowedMigGeometries struct {
	Models     []string          `yaml:"models"`
	Geometries []device.Geometry `yaml:"allowedGeometries"`
}

// These configs can be specified for each node by using Nodeconfig.
type NodeDefaultConfig struct {
	DeviceSplitCount    *uint    `yaml:"deviceSplitCount" json:"devicesplitcount"`
	DeviceMemoryScaling *float64 `yaml:"deviceMemoryScaling" json:"devicememoryscaling"`
	DeviceCoreScaling   *float64 `yaml:"deviceCoreScaling" json:"devicecorescaling"`
	// LogLevel is LIBCUDA_LOG_LEVEL value
	LogLevel *LibCudaLogLevel `yaml:"libCudaLogLevel" json:"libcudaloglevel"`
}

type NvidiaGPUDevices struct {
	config          NvidiaConfig
	ReportedGPUNum  int64
	dynamicManager  *dynamic.DynamicConfigManager
	stopDynamicChan chan struct{}
}

func InitNvidiaDevice(nvconfig NvidiaConfig) *NvidiaGPUDevices {
	klog.InfoS("initializing nvidia device", "resourceName", nvconfig.ResourceCountName, "resourceMem", nvconfig.ResourceMemoryName, "DefaultGPUNum", nvconfig.DefaultGPUNum)

	dev := &NvidiaGPUDevices{
		config:          nvconfig,
		ReportedGPUNum:  0,
		stopDynamicChan: make(chan struct{}),
	}

	// Initialize dynamic config manager if dynamic config file is specified
	if nvconfig.DynamicConfigFile != "" {
		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			klog.Warning("NODE_NAME env not set, dynamic adjustment will be disabled")
		} else {
			// Create in-cluster config
			config, err := rest.InClusterConfig()
			if err != nil {
				klog.Warningf("Failed to create in-cluster config: %v, dynamic adjustment will be disabled", err)
			} else {
				clientset, err := kubernetes.NewForConfig(config)
				if err != nil {
					klog.Warningf("Failed to create clientset: %v, dynamic adjustment will be disabled", err)
				} else {
					dev.dynamicManager = dynamic.NewDynamicConfigManager(
						nvconfig.DynamicConfigFile,
						clientset,
						nodeName,
					)

					// Load config
					if err := dev.dynamicManager.LoadConfig(); err != nil {
						klog.Warningf("Failed to load dynamic config: %v, using defaults", err)
					} else {
						// Start auto-adjustment goroutine
						go dev.dynamicManager.StartAutoAdjustment()
						klog.Info("Dynamic device adjustment enabled")
					}
				}
			}
		}
	}

	return dev
}

func (dev *NvidiaGPUDevices) CommonWord() string {
	return NvidiaGPUDevice
}

func (dev *NvidiaGPUDevices) GetNodeDevices(n *corev1.Node) ([]*device.DeviceInfo, error) {
	devEncoded, ok := n.Annotations[RegisterAnnos]
	if !ok {
		return []*device.DeviceInfo{}, errors.New("annos not found " + RegisterAnnos)
	}
	nodedevices, err := device.UnMarshalNodeDevices(devEncoded)
	if err != nil {
		klog.Infof("decode error. try to decode with old method. error %s", err.Error())
		nodedevices, err = device.DecodeNodeDevices(devEncoded)
		if err != nil {
			klog.ErrorS(err, "failed to decode node devices", "node", n.Name, "device annotation", devEncoded)
			return []*device.DeviceInfo{}, err
		}
	}
	if len(nodedevices) == 0 {
		klog.InfoS("no nvidia gpu device found", "node", n.Name, "device annotation", devEncoded)
		return []*device.DeviceInfo{}, errors.New("no gpu found on node")
	}
	for idx := range nodedevices {
		nodedevices[idx].DeviceVendor = dev.CommonWord()
	}
	for _, val := range nodedevices {
		if val.Mode == MigMode {
			val.MIGTemplate = make([]device.Geometry, 0)
			for _, migTemplates := range dev.config.MigGeometriesList {
				found := false
				for _, migDevices := range migTemplates.Models {
					if strings.Contains(val.Type, migDevices) {
						found = true
						break
					}
				}
				if found {
					val.MIGTemplate = append(val.MIGTemplate, migTemplates.Geometries...)
					break
				}
			}
		}
	}

	pairScores, ok := n.Annotations[RegisterGPUPairScore]
	if !ok {
		klog.V(5).InfoS("no topology score found", "node", n.Name)
	} else {
		devicePairScores, err := device.DecodePairScores(pairScores)
		if err != nil {
			klog.ErrorS(err, "failed to decode pair scores", "node", n.Name, "pair scores", pairScores)
			return []*device.DeviceInfo{}, err
		}
		if devicePairScores != nil {
			// fit pair score to device info
			for _, deviceInfo := range nodedevices {
				uuid := deviceInfo.ID

				for _, devicePairScore := range *devicePairScores {
					if devicePairScore.ID == uuid {
						deviceInfo.DevicePairScore = devicePairScore
						break
					}
				}
			}
		}
	}
	devDecoded := device.EncodeNodeDevices(nodedevices)
	klog.V(5).InfoS("nodes device information", "node", n.Name, "nodedevices", devDecoded)
	return nodedevices, nil
}

func (dev *NvidiaGPUDevices) GetResource(n *corev1.Node) map[string]int {
	memoryResourceName := device.GetResourceName(dev.config.ResourceMemoryName)
	coreResourceName := device.GetResourceName(dev.config.ResourceCoreName)
	memoryPercentageName := device.GetResourceName(dev.config.ResourceMemoryPercentageName)
	resourceMap := map[string]int{
		memoryResourceName:   0,
		coreResourceName:     0,
		memoryPercentageName: 0,
	}

	// Skip health check in mock mode
	if !dev.config.MockModeSkipHealthCheck {
		if !device.CheckHealthy(n, dev.config.ResourceCountName) {
			klog.Infof("device %s is unhealthy on this node", dev.CommonWord())
			return resourceMap
		}
	} else {
		klog.V(5).Infof("mock mode enabled, skipping health check for device %s", dev.CommonWord())
	}

	devs, err := dev.GetNodeDevices(n)
	if err != nil {
		// In mock mode, generate default devices if annotation not found
		if dev.config.MockModeSkipHealthCheck && dev.config.DefaultGPUNum > 0 {
			// Use dynamic config if available
			deviceCount := int(dev.config.DefaultGPUNum)
			memoryPerDevice := int(dev.config.DefaultMemory)
			coresPerDevice := int(dev.config.DefaultCores)

			if dev.dynamicManager != nil {
				dynConfig := dev.dynamicManager.GetConfig()
				if dynConfig != nil {
					deviceCount = dynConfig.DeviceCount
					memoryPerDevice = dynConfig.MemoryPerDevice
					coresPerDevice = dynConfig.CoresPerDevice
					klog.Infof("using dynamic config: %d devices, %d MB memory, %d cores each",
						deviceCount, memoryPerDevice, coresPerDevice)
				}
			}

			klog.Infof("mock mode: generating %d devices with %d MB memory, %d cores each",
				deviceCount, memoryPerDevice, coresPerDevice)

			for i := 0; i < deviceCount; i++ {
				resourceMap[memoryResourceName] += memoryPerDevice
				resourceMap[coreResourceName] += coresPerDevice
				resourceMap[memoryPercentageName] += 100
			}
		} else {
			klog.Infof("no device %s on this node", NvidiaGPUCommonWord)
			return resourceMap
		}
	} else {
		for _, val := range devs {
			resourceMap[memoryResourceName] += int(val.Devmem)
			resourceMap[coreResourceName] += int(val.Devcore)
			resourceMap[memoryPercentageName] += 100
		}
	}
	if dev.config.MemoryFactor > 1 {
		rawMemory := resourceMap[memoryResourceName]
		resourceMap[memoryResourceName] /= int(dev.config.MemoryFactor)
		klog.InfoS("Update memory", "raw", rawMemory, "after", resourceMap[memoryResourceName], "factor", dev.config.MemoryFactor)
	}
	klog.InfoS("Add resources",
		memoryResourceName,
		resourceMap[memoryResourceName],
		coreResourceName,
		resourceMap[coreResourceName],
		memoryPercentageName,
		resourceMap[memoryPercentageName],
	)
	return resourceMap
}

func (dev *NvidiaGPUDevices) RunManager() {
	lmock := mock.NewMockLister(Vendor)
	go device.Register(lmock, dev)
	mockmanager := dpm.NewManager(lmock)
	klog.Infof("Running mocking dp: %s", dev.CommonWord())
	mockmanager.Run()
}
