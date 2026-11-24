/*
Copyright 2026 Paperclip Inc.

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

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Nodes are listed once at startup to check kubelet versions; see
// DetectNativeSidecars.
// +kubebuilder:rbac:groups="",resources=nodes,verbs=list

// nativeSidecarsMinVersion is the first Kubernetes release with the
// SidecarContainers feature gate on by default. On 1.28 the gate is off by
// default: the API server strips restartPolicy from init containers and the
// kubelet rejects pods that still carry it.
var nativeSidecarsMinVersion = utilversion.MajorMinor(1, 29)

// nodeListPageSize bounds each page of the startup node listing.
const nodeListPageSize = 500

// NativeSidecarsSupported reports whether a Kubernetes component (API server
// or kubelet) with the given git version (for example "v1.28.3-eks-1234")
// supports native sidecars by default. 1.28 clusters that enabled the feature
// gate are treated as unsupported, which keeps the pre-sidecar pod layout
// there.
func NativeSidecarsSupported(gitVersion string) (bool, error) {
	v, err := utilversion.ParseGeneric(gitVersion)
	if err != nil {
		return false, fmt.Errorf("parsing Kubernetes version %q: %w", gitVersion, err)
	}
	return v.AtLeast(nativeSidecarsMinVersion), nil
}

// DetectNativeSidecars reports whether native sidecars can be used: the API
// server and the kubelet on every node must be 1.29 or later. The version
// skew policy allows 1.28 kubelets under a 1.29 API server, and a 1.28
// kubelet rejects restartable init containers, so the API server version
// alone is not enough. Detection is conservative: if nodes cannot be listed
// (for example without RBAC for nodes), there are no nodes yet, or a kubelet
// version cannot be parsed, it reports false. Only a failure to read the API
// server version, or an unexpected error listing nodes, is returned.
func DetectNativeSidecars(ctx context.Context, sv discovery.ServerVersionInterface, nodes client.Reader) (bool, error) {
	log := logf.FromContext(ctx)

	info, err := sv.ServerVersion()
	if err != nil {
		return false, fmt.Errorf("getting Kubernetes server version: %w", err)
	}
	supported, err := NativeSidecarsSupported(info.GitVersion)
	if err != nil || !supported {
		return false, err
	}

	seen := 0
	list := &corev1.NodeList{}
	for {
		opts := []client.ListOption{client.Limit(nodeListPageSize)}
		if list.Continue != "" {
			opts = append(opts, client.Continue(list.Continue))
		}
		if err := nodes.List(ctx, list, opts...); err != nil {
			if apierrors.IsForbidden(err) {
				log.Info("cannot list nodes to check kubelet versions; not using native sidecars", "error", err.Error())
				return false, nil
			}
			return false, fmt.Errorf("listing nodes: %w", err)
		}
		for i := range list.Items {
			node := &list.Items[i]
			kubelet := node.Status.NodeInfo.KubeletVersion
			ok, err := NativeSidecarsSupported(kubelet)
			if err != nil || !ok {
				log.Info("node kubelet does not support native sidecars; not using them",
					"node", node.Name, "kubeletVersion", kubelet)
				return false, nil
			}
		}
		seen += len(list.Items)
		if list.Continue == "" {
			break
		}
	}
	if seen == 0 {
		log.Info("no nodes found to check kubelet versions; not using native sidecars")
		return false, nil
	}
	return true, nil
}
