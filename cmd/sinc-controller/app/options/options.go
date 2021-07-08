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

package options

import (
	"github.com/spf13/pflag"
)

type Options struct {
	MasterUrl     string
	K8sConfigPath string
}

func NewSincControllerOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}
	fs.StringVar(&o.MasterUrl, "master-url", o.MasterUrl, "kube master url")
	fs.StringVar(&o.K8sConfigPath, "k8sconfig-path", o.K8sConfigPath, "kuconfig path")
}

func (o *Options) Validate() error {
	return nil
}
