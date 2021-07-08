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

package bgpnumber

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/superedge/superedge/pkg/sinc-controller/constants"
	asset "github.com/superedge/superedge/pkg/sinc-controller/node/bgpnumber/as__set"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	informers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	nodeutil "k8s.io/kubernetes/pkg/controller/util/node"
)

const (
	asUpdateQueueSize = 5000

	asNumberUpdateWorkers = 10

	asUpdateRetries = 3
)

type NodeAllocator interface {
	AllocateOrOccupyASNumber(node *v1.Node) error

	ReleaseASNumber(node *v1.Node) error

	Run(stopCh <-chan struct{})
}

type ASAllocator struct {
	client clientset.Interface

	ASSet *asset.AsSet
	// nodeLister is able to list/get nodes and is populated by the shared informer passed to controller
	nodeLister corelisters.NodeLister
	// nodesSynced returns true if the node shared informer has been synced at least once.
	nodesSynced cache.InformerSynced
	// Channel that is used to pass updating Nodes and their reserved AS to the background
	nodeASUpdateChannel chan nodeReservedAS
	lock                sync.Mutex
	nodesInProcessing   sets.String
}

type nodeReservedAS struct {
	allocatedAS int
	nodeName    string
}

// AsAllocatorParams is parameters that's required for creating new
// AS Number allocator.
type AsAllocatorParams struct {
	// AsStart is start of As Range
	AsStart int
	// MaxAsCount is max As Count
	MaxAsCount int
}

// NewAsNumberAllocator returns a CIDRAllocator to allocate CIDRs for node
// Caller must always pass in a list of existing nodes so the new allocator.
func NewAsNumberAllocator(client clientset.Interface, nodeInformer informers.NodeInformer, allocatorParams AsAllocatorParams, nodeList *v1.NodeList) (NodeAllocator, error) {
	if client == nil {
		klog.Fatalf("kubeClient is nil when starting NodeController")
	}

	asSet, err := asset.NewASSet(allocatorParams.AsStart, allocatorParams.MaxAsCount)
	if err != nil {
		return nil, fmt.Errorf("init as set failed :%s", err.Error())
	}

	ra := &ASAllocator{
		client:              client,
		ASSet:               asSet,
		nodeLister:          nodeInformer.Lister(),
		nodesSynced:         nodeInformer.Informer().HasSynced,
		nodeASUpdateChannel: make(chan nodeReservedAS, asUpdateQueueSize),
		nodesInProcessing:   sets.NewString(),
	}

	if nodeList != nil {
		for _, node := range nodeList.Items {
			var tarAs string
			var ok bool
			if tarAs, ok = node.Labels[constants.BgpAsNumber]; !ok {
				klog.V(4).Infof("Node %v has no AsNumber, ignoring", node.Name)
				continue
			}
			klog.V(4).Infof("Node %v has As Number %s, occupying it", node.Name, tarAs)
			tarAsInt, err := strconv.Atoi(tarAs)
			if err != nil {
				klog.Warningf("Node %s has As Number %s, transfer to int failed:%s", node.Name, tarAs, err.Error())
				continue
			}
			if err := ra.occupyAsNumber(tarAsInt, node.Name); err != nil {
				return nil, err
			}
		}
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: nodeutil.CreateAddNodeHandler(ra.AllocateOrOccupyASNumber),
		UpdateFunc: nodeutil.CreateUpdateNodeHandler(func(_, newNode *v1.Node) error {
			if _, ok := newNode.Labels[constants.BgpAsNumber]; !ok {
				return ra.AllocateOrOccupyASNumber(newNode)
			}
			return nil
		}),
		DeleteFunc: nodeutil.CreateDeleteNodeHandler(ra.ReleaseASNumber),
	})

	return ra, nil
}

func (r *ASAllocator) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	klog.Infof("Starting bgp As Number allocator")
	defer klog.Infof("Shutting down bgp As Number allocator")

	if !cache.WaitForNamedCacheSync("bgp asNumber allocator", stopCh, r.nodesSynced) {
		return
	}

	for i := 0; i < asNumberUpdateWorkers; i++ {
		go r.worker(stopCh)
	}

	<-stopCh
}

// WARNING: If you're adding any return calls or defer any more work from this
// function you have to make sure to update nodesInProcessing properly with the
// disposition of the node when the work is done.
func (r *ASAllocator) AllocateOrOccupyASNumber(node *v1.Node) error {
	if node == nil {
		return nil
	}
	if !r.insertNodeToProcessing(node.Name) {
		klog.V(4).Infof("Node %v is already in a process of asNumber assignment.", node.Name)
		return nil
	}

	if tarAs, ok := node.Labels[constants.BgpAsNumber]; ok {
		tarAsInt, err := strconv.Atoi(tarAs)
		if err != nil {
			klog.Warningf("Node %s has As Number %s, transfer to int failed:%s", node.Name, tarAs, err.Error())
			return err
		}
		return r.occupyAsNumber(tarAsInt, node.Name)
	}
	// allocate and queue the assignment
	allocated := nodeReservedAS{
		nodeName: node.Name,
	}

	saveAs, err := r.ASSet.AllocateNext()
	if err != nil {
		r.removeNodeFromProcessing(node.Name)
		return fmt.Errorf("failed to allocate bgpNumber from node:%s", err.Error())
	}
	allocated.allocatedAS = saveAs

	//queue the assignment
	klog.V(4).Infof("Putting node %s with asNumber %d into the work queue", node.Name, allocated.allocatedAS)
	r.nodeASUpdateChannel <- allocated
	return nil
}

// ReleaseASNumber marks node.podCIDRs[...] as unused in our tracked cidrSets
func (r *ASAllocator) ReleaseASNumber(node *v1.Node) error {
	var tarAs string
	var ok bool
	klog.V(4).Infof("receive node delete event %s", node.Name)
	if node == nil {
		return nil
	}

	if tarAs, ok = node.Labels[constants.BgpAsNumber]; !ok {
		klog.V(4).Infof("node %s has no as number occupy,no need to release", node.Name)
		return nil
	}

	klog.V(4).Infof("release asNumber %s for node:%v", tarAs, node.Name)
	tarAsInt, err := strconv.Atoi(tarAs)
	if err != nil {
		klog.Warningf("Node %s has As Number %s, transfer to int failed:%s", node.Name, tarAs, err.Error())
		return err
	}
	if err = r.ASSet.Release(tarAsInt); err != nil {
		return fmt.Errorf("error when releasing asNumber %s, %v", tarAs, err)
	}

	return nil
}

func (r *ASAllocator) worker(stopChan <-chan struct{}) {
	for {
		select {
		case workItem, ok := <-r.nodeASUpdateChannel:
			if !ok {
				klog.Warning("Channel nodeASUpdateChannel was unexpectedly closed")
				return
			}
			if err := r.updateASAllocation(workItem); err != nil {
				// Requeue the failed node for update again.
				r.nodeASUpdateChannel <- workItem
			}
		case <-stopChan:
			return
		}
	}
}

// updateASAllocation assigns AS to Node and sends an update to the API server.
func (r *ASAllocator) updateASAllocation(data nodeReservedAS) error {
	var err error
	var node *v1.Node
	var tarAs string
	var ok bool
	defer r.removeNodeFromProcessing(data.nodeName)

	node, err = r.nodeLister.Get(data.nodeName)
	if err != nil {
		klog.Errorf("Failed while getting node %v for updating Node.Spec.PodCIDRs: %v", data.nodeName, err)
		return err
	}

	if tarAs, ok = node.Labels[constants.BgpAsNumber]; ok {
		if tarAs == strconv.Itoa(data.allocatedAS) {
			klog.V(4).Infof("Node %s already has allocated asNumber %v. It matches the proposed one.", node.Name, data.allocatedAS)
			return nil
		}
	}

	// has bgp AsNumber label but diff,need to release new one
	if len(tarAs) != 0 {
		klog.Errorf("Node %v already has a asNumber allocated %s. Releasing the new one.", node.Name, tarAs)

		if releaseErr := r.ASSet.Release(data.allocatedAS); releaseErr != nil {
			klog.Errorf("Error when releasing asNumber:%v, err:%v", data.allocatedAS, releaseErr)
		}

		return nil
	}

	var patahContent = map[string]interface{}{"metadata": map[string]map[string]string{
		"labels": {constants.BgpAsNumber: strconv.Itoa(data.allocatedAS)}}}
	patchData, err := json.Marshal(patahContent)
	if err != nil {
		klog.Error("marshal patch data failed：%s", err.Error())
		return fmt.Errorf("marshal patch data failed：%s", err.Error())
	}

	// If we reached here, it means that the node has no AS Number currently assigned. So we set it.
	for i := 0; i < asUpdateRetries; i++ {
		if _, err = r.client.CoreV1().Nodes().Patch(context.Background(), data.nodeName,
			types.StrategicMergePatchType, patchData,
			metav1.PatchOptions{}); err == nil {
			klog.Infof("Set node %v AsNumber to %v", node.Name, strconv.Itoa(data.allocatedAS))
			return nil
		}
	}

	return err
}

// occupyCIDRs
func (r *ASAllocator) occupyAsNumber(tarAs int, nodeName string) error {
	defer r.removeNodeFromProcessing(nodeName)

	if err := r.ASSet.Occupy(tarAs); err != nil {
		return fmt.Errorf("failed to mark %d as occupied for node: %v: %v", tarAs, nodeName, err.Error())
	}

	return nil
}

func (r *ASAllocator) insertNodeToProcessing(nodeName string) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.nodesInProcessing.Has(nodeName) {
		return false
	}
	r.nodesInProcessing.Insert(nodeName)
	return true
}

func (r *ASAllocator) removeNodeFromProcessing(nodeName string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.nodesInProcessing.Delete(nodeName)
}
