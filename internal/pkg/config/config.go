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

package config

import (
	"context"
	"flag"
	"os"

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/amd"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/ascend"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/awsneuron"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/cambricon"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/enflame"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/hygon"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/iluvatar"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/kunlun"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/metax"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/mthreads"
	"github.com/HAMi/mock-device-plugin/internal/pkg/api/device/nvidia"
	"github.com/HAMi/mock-device-plugin/internal/pkg/util/client"
)

type Config struct {
	NvidiaConfig    nvidia.NvidiaConfig       `yaml:"nvidia"`
	MetaxConfig     metax.MetaxConfig         `yaml:"metax"`
	HygonConfig     hygon.HygonConfig         `yaml:"hygon"`
	CambriconConfig cambricon.CambriconConfig `yaml:"cambricon"`
	MthreadsConfig  mthreads.MthreadsConfig   `yaml:"mthreads"`
	IluvatarConfig  []iluvatar.IluvatarConfig `yaml:"iluvatars"`
	EnflameConfig   enflame.EnflameConfig     `yaml:"enflame"`
	KunlunConfig    kunlun.KunlunConfig       `yaml:"kunlun"`
	AWSNeuronConfig awsneuron.AWSNeuronConfig `yaml:"awsneuron"`
	AMDGPUConfig    amd.AMDConfig             `yaml:"amd"`
	VNPUs           []ascend.VNPUConfig       `yaml:"vnpus"`
}

var (
	configFile string
)

func LoadConfig(path string) (*Config, error) {
	klog.Infof("Reading config file from path: %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var yamlData Config
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return nil, err
	}
	klog.Info("Successfully read and parsed config file")
	return &yamlData, nil
}

// getAcceleratorLabel reads the "accelerator" label from the current node.
// Returns the label value and true if found, or empty string and false otherwise.
func getAcceleratorLabel() (string, bool) {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Warning("NODE_NAME env not set, cannot determine accelerator type")
		return "", false
	}

	kubeClient := client.GetClient()
	node, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get node %s: %v", nodeName, err)
		return "", false
	}

	val, found := node.Labels["accelerator"]
	if !found {
		klog.Infof("Node %s does not have 'accelerator' label", nodeName)
		return "", false
	}

	klog.Infof("Node %s has accelerator label: %s", nodeName, val)
	return val, true
}

func InitDevicesWithConfig(config *Config) error {
	device.DevicesMap = make(map[string]device.Devices)

	// Determine which devices to initialize based on the "accelerator" node label
	acceleratorType, hasLabel := getAcceleratorLabel()

	if !hasLabel {
		klog.Warning("No accelerator label found on node, initializing all configured devices")
		// Fallback: initialize all configured devices (original behavior)
		for _, dev := range ascend.InitDevices(config.VNPUs) {
			commonWord := dev.CommonWord()
			device.DevicesMap[commonWord] = dev
			klog.Infof("Ascend device %s initialized", commonWord)
		}
		hygonDevice := hygon.InitDCUDevice(config.HygonConfig)
		if hygonDevice != nil {
			device.DevicesMap[hygonDevice.CommonWord()] = hygonDevice
		}
		nvidiaDevice := nvidia.InitNvidiaDevice(config.NvidiaConfig)
		if nvidiaDevice != nil {
			device.DevicesMap[nvidiaDevice.CommonWord()] = nvidiaDevice
		}
		return nil
	}

	// Only initialize devices matching the accelerator label
	switch acceleratorType {
	case "nvidia":
		klog.Info("Accelerator type is nvidia, initializing NVIDIA GPU devices only")
		nvidiaDevice := nvidia.InitNvidiaDevice(config.NvidiaConfig)
		if nvidiaDevice != nil {
			device.DevicesMap[nvidiaDevice.CommonWord()] = nvidiaDevice
		}
	case "huawei-Ascend910":
		klog.Info("Accelerator type is huawei-Ascend910, initializing Ascend VNPU devices only")
		for _, dev := range ascend.InitDevices(config.VNPUs) {
			commonWord := dev.CommonWord()
			device.DevicesMap[commonWord] = dev
			klog.Infof("Ascend device %s initialized", commonWord)
		}
	default:
		klog.Warningf("Unknown accelerator type %q, initializing all configured devices", acceleratorType)
		for _, dev := range ascend.InitDevices(config.VNPUs) {
			commonWord := dev.CommonWord()
			device.DevicesMap[commonWord] = dev
			klog.Infof("Ascend device %s initialized", commonWord)
		}
		hygonDevice := hygon.InitDCUDevice(config.HygonConfig)
		if hygonDevice != nil {
			device.DevicesMap[hygonDevice.CommonWord()] = hygonDevice
		}
		nvidiaDevice := nvidia.InitNvidiaDevice(config.NvidiaConfig)
		if nvidiaDevice != nil {
			device.DevicesMap[nvidiaDevice.CommonWord()] = nvidiaDevice
		}
	}

	return nil
}

func InitDevices() {
	if len(device.DevicesMap) > 0 {
		klog.Info("Devices are already initialized, skipping initialization")
		return
	}
	klog.Infof("Loading device configuration from file: %s", configFile)
	config, err := LoadConfig(configFile)
	if err != nil {
		klog.Fatalf("Failed to load device config file %s: %v", configFile, err)
	}
	klog.Infof("Loaded config: %v", config)
	err = InitDevicesWithConfig(config)
	if err != nil {
		klog.Fatalf("Failed to initialize devices: %v", err)
	}
}

func GlobalFlagSet() {
	flag.StringVar(&configFile, "device-config-file", "", "Path to the device config file")
}
