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

package configmap

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/superedge/superedge/pkg/sinc-controller/constants"
	"github.com/superedge/superedge/pkg/sinc-controller/node"
	"github.com/superedge/superedge/pkg/sinc-controller/types"

	v1 "k8s.io/api/core/v1"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var CacheEdgeZones ConfigMapList

type ConfigMapList struct {
	EdgeZoneMap    map[string]EdgeZoneInfo
	ConfigMapMutex sync.Mutex
}

type EdgeZoneInfo struct {
	Name     string `json:"name,"`
	Type     string `json:"type,"`
	Cidr     string `json:"cidr,"`
	NodeMask int    `json:"node_mask,"`

	VendorName string `json:"vendor_name,omitempty"`
	KeyId      string `json:"key_id,omitempty"`
	SecretId   string `json:"secret_id,omitempty"`
	Region     string `json:"region,omitempty"`
	SubnetId   string `json:"subnet_id,omitempty"`
	SGroupId   string `json:"sgroup_id,omitempty"`
	VpcId      string `json:"vpc_id,omitempty"`

	Tor          string `json:"tor,omitempty"`
	TorAs        uint32 `json:"tor_as,omitempty"`
	NodeAsStart  int    `json:"node_as_start,omitempty"`
	BgpInterface string `json:"bgp_interface,omitempty"`

	Cancel context.CancelFunc
}

type ConfigMapController struct {
	clientset             kubernetes.Interface
	ConfigMapInformer     coreinformers.ConfigMapInformer
	ConfigMapLister       corelisters.ConfigMapLister
	ConfigMapListerSynced cache.InformerSynced
}

func init() {
	var configMapList = make(map[string]EdgeZoneInfo, 3)
	var configMapMutex sync.Mutex
	CacheEdgeZones = ConfigMapList{
		EdgeZoneMap:    configMapList,
		ConfigMapMutex: configMapMutex,
	}
}

func NewConfigMapController(clientset kubernetes.Interface) *ConfigMapController {
	SharedInformerFactory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	configMapInformer := SharedInformerFactory.Core().V1().ConfigMaps()
	n := &ConfigMapController{}
	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: n.handleConfigMapAddUpdate,
		UpdateFunc: func(old, cur interface{}) {
			n.handleConfigMapAddUpdate(cur)
		},
		DeleteFunc: n.handleConfigMapDelete,
	})
	n.clientset = clientset
	n.ConfigMapInformer = configMapInformer
	n.ConfigMapLister = configMapInformer.Lister()
	n.ConfigMapListerSynced = configMapInformer.Informer().HasSynced
	return n
}

func (n *ConfigMapController) handleConfigMapAddUpdate(obj interface{}) {
	configMap, ok := obj.(*v1.ConfigMap)
	if !ok {
		return
	}
	if configMap.Namespace == constants.SincNS {
		klog.Infof("get a sinc configMap add or update event,configMap name:%s", configMap.Name)
		if edgeRoomBinary, ok := configMap.Data[constants.ZoneInfoKey]; ok {
			var edgeRoom EdgeZoneInfo
			err := json.Unmarshal([]byte(edgeRoomBinary), &edgeRoom)
			if err != nil {
				klog.Errorf("parse configmap %s failed ：%s", configMap.Name, err.Error())
				return
			}
			CacheEdgeZones.ConfigMapMutex.Lock()
			if _, ok := CacheEdgeZones.EdgeZoneMap[edgeRoom.Name]; !ok {
				klog.Infof("first time create for edgeZone %s,begin to init node controller", edgeRoom.Name)
				ctx, cancel := context.WithCancel(context.Background())
				var zone = types.ZoneInfo{
					Name:        edgeRoom.Name,
					Type:        edgeRoom.Type,
					Cidr:        edgeRoom.Cidr,
					NodeMask:    edgeRoom.NodeMask,
					Tor:         edgeRoom.Tor,
					TorAs:       edgeRoom.TorAs,
					NodeAsStart: edgeRoom.NodeAsStart,
					Ctx:         ctx,
				}
				node.ZoneChannel <- zone
				edgeRoom.Cancel = cancel
				CacheEdgeZones.EdgeZoneMap[edgeRoom.Name] = edgeRoom
			}
			CacheEdgeZones.ConfigMapMutex.Unlock()
		}
	}
}

func (n *ConfigMapController) handleConfigMapDelete(obj interface{}) {
	configMap, ok := obj.(*v1.ConfigMap)
	if !ok {
		return
	}
	if configMap.Namespace == constants.SincNS {
		klog.Infof("get a configMap del event,configMap name:%s", configMap.Name)
		CacheEdgeZones.ConfigMapMutex.Lock()
		if edgeRoom, ok := CacheEdgeZones.EdgeZoneMap[configMap.Name]; ok {
			klog.Infof("stop node controller and delete cache edge zone")
			edgeRoom.Cancel()
			delete(CacheEdgeZones.EdgeZoneMap, configMap.Name)
		}
		CacheEdgeZones.ConfigMapMutex.Unlock()
	}
}

func (n *ConfigMapController) Run(ctx context.Context) {
	defer runtimeutil.HandleCrash()

	go n.ConfigMapInformer.Informer().Run(ctx.Done())

	if ok := cache.WaitForCacheSync(
		ctx.Done(),
		n.ConfigMapListerSynced,
	); !ok {
		log.Fatal("failed to wait for caches to sync")
	}

	<-ctx.Done()
}
