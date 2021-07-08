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

package daemon

import (
	"context"

	"github.com/superedge/superedge/pkg/sinc-controller/configmap"
	"github.com/superedge/superedge/pkg/sinc-controller/node"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func Run(clientSet *kubernetes.Clientset, ctx context.Context) {
	configMapController := configmap.NewConfigMapController(clientSet)

	go configMapController.Run(ctx)

	klog.Fatal(node.StartNodeController(clientSet, ctx))
}
