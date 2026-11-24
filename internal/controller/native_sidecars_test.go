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
	"testing"

	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestNativeSidecarsSupported(t *testing.T) {
	tests := []struct {
		gitVersion string
		want       bool
	}{
		{"v1.28.0", false},
		{"v1.28.15-eks-a737599", false},
		{"v1.28.9-gke.1000000", false},
		{"v1.29.0", true},
		{"v1.29.4+k3s1", true},
		{"v1.31.2", true},
		{"v2.0.0", true},
	}
	for _, tt := range tests {
		got, err := NativeSidecarsSupported(tt.gitVersion)
		if err != nil {
			t.Fatalf("NativeSidecarsSupported(%q) error: %v", tt.gitVersion, err)
		}
		if got != tt.want {
			t.Errorf("NativeSidecarsSupported(%q) = %v, want %v", tt.gitVersion, got, tt.want)
		}
	}

	if _, err := NativeSidecarsSupported("not-a-version"); err == nil {
		t.Error("expected an error for an unparsable version")
	}
}

func TestDetectNativeSidecars(t *testing.T) {
	for gitVersion, want := range map[string]bool{"v1.28.15": false, "v1.29.0": true} {
		client := &fakediscovery.FakeDiscovery{
			Fake:               &clienttesting.Fake{},
			FakedServerVersion: &version.Info{GitVersion: gitVersion},
		}
		got, err := DetectNativeSidecars(client)
		if err != nil {
			t.Fatalf("DetectNativeSidecars(%s) error: %v", gitVersion, err)
		}
		if got != want {
			t.Errorf("DetectNativeSidecars(%s) = %v, want %v", gitVersion, got, want)
		}
	}
}
