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
	"fmt"

	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/discovery"
)

// nativeSidecarsMinVersion is the first Kubernetes release with the
// SidecarContainers feature gate on by default. On 1.28 the gate is off by
// default and the API server strips restartPolicy from init containers.
var nativeSidecarsMinVersion = utilversion.MajorMinor(1, 29)

// NativeSidecarsSupported reports whether a Kubernetes server with the given
// git version (for example "v1.28.3-eks-1234") supports native sidecars by
// default. 1.28 clusters that enabled the feature gate are treated as
// unsupported, which keeps the pre-sidecar pod layout there.
func NativeSidecarsSupported(gitVersion string) (bool, error) {
	v, err := utilversion.ParseGeneric(gitVersion)
	if err != nil {
		return false, fmt.Errorf("parsing Kubernetes server version %q: %w", gitVersion, err)
	}
	return v.AtLeast(nativeSidecarsMinVersion), nil
}

// DetectNativeSidecars asks the API server for its version and reports
// whether native sidecars are supported.
func DetectNativeSidecars(client discovery.ServerVersionInterface) (bool, error) {
	info, err := client.ServerVersion()
	if err != nil {
		return false, fmt.Errorf("getting Kubernetes server version: %w", err)
	}
	return NativeSidecarsSupported(info.GitVersion)
}
