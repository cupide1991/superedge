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

package app

import (
	"context"

	"github.com/superedge/superedge/cmd/sinc-controller/app/options"
	daemon "github.com/superedge/superedge/pkg/sinc-controller"
	"github.com/superedge/superedge/pkg/util"
	"github.com/superedge/superedge/pkg/version"
	"github.com/superedge/superedge/pkg/version/verflag"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var appName = "sinc-controller"

func NewSincControllerCommand(ctx context.Context) *cobra.Command {
	o := options.NewSincControllerOptions()
	cmd := &cobra.Command{
		Use: appName,
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()

			klog.Infof("Versions: %#v\n", version.Get())
			util.PrintFlags(cmd.Flags())

			if err := o.Validate(); err != nil {
				klog.Fatalf("sinc-controller start with err %s", err.Error())
			}

			kubeconfig, err := clientcmd.BuildConfigFromFlags(o.MasterUrl, o.K8sConfigPath)
			if err != nil {
				klog.Fatalf("failed to create kubeconfig: %v", err)
			}

			clientSet, err := kubernetes.NewForConfig(kubeconfig)
			if err != nil {
				klog.Fatalf("failed to init clientSet: %v", err)
			}

			daemon.Run(clientSet, ctx)
			klog.Fatal("sinc-controller end")
		},
	}
	fs := cmd.Flags()
	o.AddFlags(fs)

	return cmd
}
