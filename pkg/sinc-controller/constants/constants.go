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

package constants

const (
	ReCodeBadRequest = -1
	ReCodeOK         = 0

	DefaultNodeSocket     = "/var/run/sinc-node/sinc_cni.socket"
	DefaultNodeSocketPath = "/var/run/sinc-node/"
	DefaultNodeSocketFile = "sinc_cni.socket"
	DefaultBufSize        = 10240

	// four node types
	Node    = "node"
	MixNode = "mix_node"
	TorNode = "tor_node"
	VPCNode = "vpc_node"

	// for ip allocator
	DefaultNetworkName = "sinc-edge"
	DefaultNetworkDir  = "/var/lib/network"

	// three cloud vendor
	VENDOR_AWS = "AWS"
	VENDOR_ALI = "ALIYUN"
	VENDOR_TC  = "TENCENT"

	// 移动，电信，联通，腾讯云边缘云的internetServiceProvider
	CMCC = "CMCC"
	CTCC = "CTCC"
	CUCC = "CUCC"

	EthernetMTU       = 1500
	DefaultRTTable    = "16"
	DefaultRTTableInt = 16
	ZoneInfoKey       = "edgeroom.json"
	SincNS            = "sinc"
	EdgeTopologyZone  = "sincedge.io/edge-topology-zone"
	BgpAsNumber       = "sincedge.io/bgp-as-number"
	EdgeNodeType      = "sincedge.io/node-type"
	BgpRouterId       = "sincedge.io/bgp-router-id"
	MaxBgpCount       = 1024
	TypeClassic       = "classic"
	TypeVPC           = "vpc"
	MaxParrZones      = 20
	TelnetServer      = "127.0.0.1:2605"
)
