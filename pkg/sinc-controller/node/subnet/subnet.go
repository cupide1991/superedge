/*
Copyright 2021 The SuperEdge Authors.

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

package subnet

import (
	"fmt"
	"net"

	"github.com/superedge/superedge/pkg/sinc-controller/types"

	v1 "k8s.io/api/core/v1"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/controller/nodeipam/ipam"
)

func NewNodeIpamController(clientSet *kubernetes.Clientset, nodeInformer informers.NodeInformer, zone types.ZoneInfo, nodeList *v1.NodeList) (ipam.CIDRAllocator, error) {
	_, cidr, err := net.ParseCIDR(zone.Cidr)
	if err != nil {
		return nil, fmt.Errorf("parse cidr %s failed:%s", zone.Cidr, err.Error())
	}
	var allocatorParams = ipam.CIDRAllocatorParams{
		ClusterCIDRs:      []*net.IPNet{cidr},
		NodeCIDRMaskSizes: []int{zone.NodeMask},
	}

	controller, err := ipam.NewCIDRRangeAllocator(clientSet, nodeInformer, allocatorParams, nodeList)
	if err != nil {
		klog.Errorf("new ipam node controller failed:%s", err.Error())
		return nil, fmt.Errorf("new ipam node controller failed:%s", err.Error())
	}

	return controller, nil
}
