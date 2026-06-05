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

package mock

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"k8s.io/klog/v2"
	kubeletdevicepluginv1beta1 "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	// maxMemoryDeviceCount is the maximum number of Device objects to create
	// for memory-type resources. Memory resources use large count values (e.g.,
	// 131072 for total MB) that would exceed kubelet's 4MB gRPC message limit.
	maxMemoryDeviceCount = 128
)

// Plugin is identical to DevicePluginServer interface of device plugin API.
type MockPlugin struct {
	ManagedResource string
	count           atomic.Int64
}

// Start is an optional interface that could be implemented by plugin.
// If case Start is implemented, it will be executed by Manager after
// plugin instantiation and before its registration to kubelet. This
// method could be used to prepare resources before they are offered
// to Kubernetes.
func (p *MockPlugin) Start() error {
	klog.Infoln("Mock manager start")
	return nil
}

// Stop is an optional interface that could be implemented by plugin.
// If case Stop is implemented, it will be executed by Manager after the
// plugin is unregistered from kubelet. This method could be used to tear
// down resources.
func (p *MockPlugin) Stop() error {
	return nil
}

// GetDevicePluginOptions returns options to be communicated with Device
// Manager
func (p *MockPlugin) GetDevicePluginOptions(ctx context.Context, e *kubeletdevicepluginv1beta1.Empty) (*kubeletdevicepluginv1beta1.DevicePluginOptions, error) {
	return &kubeletdevicepluginv1beta1.DevicePluginOptions{}, nil
}

// PreStartContainer is expected to be called before each container start if indicated by plugin during registration phase.
// PreStartContainer allows kubelet to pass reinitialized devices to containers.
// PreStartContainer allows Device Plugin to run device specific operations on the Devices requested
func (p *MockPlugin) PreStartContainer(ctx context.Context, r *kubeletdevicepluginv1beta1.PreStartContainerRequest) (*kubeletdevicepluginv1beta1.PreStartContainerResponse, error) {
	return &kubeletdevicepluginv1beta1.PreStartContainerResponse{}, nil
}

// GetPreferredAllocation returns a preferred set of devices to allocate
// from a list of available ones. The resulting preferred allocation is not
// guaranteed to be the allocation ultimately performed by the
// devicemanager. It is only designed to help the devicemanager make a more
// informed allocation decision when possible.
func (p *MockPlugin) GetPreferredAllocation(context.Context, *kubeletdevicepluginv1beta1.PreferredAllocationRequest) (*kubeletdevicepluginv1beta1.PreferredAllocationResponse, error) {
	return &kubeletdevicepluginv1beta1.PreferredAllocationResponse{}, nil
}

// ListAndWatch returns a stream of List of Devices
// Whenever a Device state change or a Device disappears, ListAndWatch
// returns the new list
func (p *MockPlugin) ListAndWatch(e *kubeletdevicepluginv1beta1.Empty, s kubeletdevicepluginv1beta1.DevicePlugin_ListAndWatchServer) error {
	for {
		count := p.GetCount()
		// For memory-type resources, the count value represents total MB
		// (e.g., 131072 for 4x32768MB Ascend910-memory), not the number of
		// physical devices. Creating that many Device objects would exceed
		// kubelet's gRPC max message size (4MB), causing the plugin to be
		// deregistered. Limit the device count to a reasonable number for
		// memory-type resources to avoid gRPC message size overflow.
		deviceCount := count
		if isMemoryResource(p.ManagedResource) && count > maxMemoryDeviceCount {
			deviceCount = maxMemoryDeviceCount
			klog.Infof("Limiting memory resource %s device count from %d to %d to avoid gRPC message overflow",
				p.ManagedResource, count, deviceCount)
		}
		devs := make([]*kubeletdevicepluginv1beta1.Device, deviceCount)
		for i := 0; i < deviceCount; i++ {
			devs[i] = &kubeletdevicepluginv1beta1.Device{
				ID:     fmt.Sprintf("mock-devices-id-%d", i),
				Health: kubeletdevicepluginv1beta1.Healthy,
			}
		}
		klog.Infoln("Device Registered", p.ManagedResource, deviceCount)
		s.Send(&kubeletdevicepluginv1beta1.ListAndWatchResponse{Devices: devs})
		time.Sleep(time.Second * 10)
	}
}

func (p *MockPlugin) Allocate(ctx context.Context, reqs *kubeletdevicepluginv1beta1.AllocateRequest) (*kubeletdevicepluginv1beta1.AllocateResponse, error) {
	var response kubeletdevicepluginv1beta1.AllocateResponse
	var car kubeletdevicepluginv1beta1.ContainerAllocateResponse

	klog.Infoln("Into Allocate")
	for range reqs.ContainerRequests {
		car = kubeletdevicepluginv1beta1.ContainerAllocateResponse{}
		response.ContainerResponses = append(response.ContainerResponses, &car)
	}

	return &response, nil
}

func (p *MockPlugin) GetCount() int {
	return int(p.count.Load())
}

func (p *MockPlugin) SetCount(count int) {
	p.count.Store(int64(count))
}

// isMemoryResource checks if the resource name represents a memory-type resource.
// Memory resources have count values in MB (e.g., 131072) rather than device counts,
// and need special handling to avoid creating too many Device objects.
func isMemoryResource(resourceName string) bool {
	return strings.Contains(strings.ToLower(resourceName), "memory") ||
		strings.Contains(strings.ToLower(resourceName), "mem")
}
