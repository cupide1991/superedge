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

package node

import (
	"context"
	"fmt"
	"time"

	"github.com/superedge/superedge/pkg/sinc-controller/constants"
	"github.com/superedge/superedge/pkg/sinc-controller/node/bgpnumber"
	"github.com/superedge/superedge/pkg/sinc-controller/node/subnet"
	"github.com/superedge/superedge/pkg/sinc-controller/types"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/controller/nodeipam/ipam"
)

var ZoneChannel = make(chan types.ZoneInfo, constants.MaxParrZones)

func StartNodeController(clientSet *kubernetes.Clientset, ctx context.Context) error {
	for {
		select {
		case workItem, ok := <-ZoneChannel:
			if !ok {
				klog.Warning("Channel zoneChannel was unexpectedly closed")
				return fmt.Errorf("Channel zoneChannel was unexpectedly closed ")
			}
			if err := StartNodeControllerByZone(workItem, clientSet, workItem.Ctx); err != nil {
				// Requeue the failed zone for update again.
				return fmt.Errorf("start node controller with zoneInfo %v failed:%s", workItem, err.Error())
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func StartNodeControllerByZone(zone types.ZoneInfo, clientSet *kubernetes.Clientset, ctx context.Context) error {
	var asAllocator bgpnumber.NodeAllocator
	var subnetAllocator ipam.CIDRAllocator
	var err error
	labelSelector := labels.NewSelector()
	zoneRequirement, _ := labels.NewRequirement(constants.EdgeTopologyZone, selection.Equals, []string{zone.Name})
	labelSelector = labelSelector.Add(*zoneRequirement)
	options := informers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.LabelSelector = labelSelector.String()
	})

	SharedInformerFactory := informers.NewSharedInformerFactoryWithOptions(clientSet, 10*time.Minute, options)
	nodeInformer := SharedInformerFactory.Core().V1().Nodes()

	nodeList := getNodeList(ctx, clientSet, labelSelector)

	klog.Infof("start node controller for zone %s", zone.Name)
	subnetAllocator, err = subnet.NewNodeIpamController(clientSet, nodeInformer, zone, nodeList)
	if err != nil {
		klog.Errorf("new asNumber allocator for zone %s  failed:%s", err.Error(), zone.Name)
		return fmt.Errorf("new asNumber allocator for zone %s failed:%s", err.Error(), zone.Name)
	}

	go nodeInformer.Informer().Run(ctx.Done())

	go subnetAllocator.Run(ctx.Done())

	if zone.Type == constants.TypeClassic {
		var bgpAsNumberParas = bgpnumber.AsAllocatorParams{
			AsStart:    zone.NodeAsStart,
			MaxAsCount: constants.MaxBgpCount,
		}
		asAllocator, err = bgpnumber.NewAsNumberAllocator(clientSet, nodeInformer, bgpAsNumberParas, nodeList)
		if err != nil {
			klog.Errorf("new asNumber allocator for zone %s failed:%s", err.Error(), zone.Name)
			return fmt.Errorf("new asNumber allocator for zone %s failed:%s", err.Error(), zone.Name)
		}
		go asAllocator.Run(ctx.Done())
	}

	return nil
}

func getNodeList(ctx context.Context, clientSet *kubernetes.Clientset, listSelector labels.Selector) *v1.NodeList {
	options := metav1.ListOptions{
		LabelSelector: listSelector.String(),
	}
	nodeList, err := clientSet.CoreV1().Nodes().List(ctx, options)
	if err != nil {
		klog.Errorf("list node failed:%s", err.Error())
		return nil
	}

	for index, node := range nodeList.Items {
		klog.Infof("zone %s list node %d :  %s", listSelector.String(), index+1, node.Name)
	}
	return nodeList
}
