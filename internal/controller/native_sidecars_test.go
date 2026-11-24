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
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
	node := func(name, kubelet string) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{KubeletVersion: kubelet}},
		}
	}
	tests := []struct {
		name       string
		apiServer  string
		nodes      []*corev1.Node
		forbidNode bool
		want       bool
	}{
		{name: "1.28 cluster", apiServer: "v1.28.15", nodes: []*corev1.Node{node("a", "v1.28.15")}, want: false},
		{name: "1.29 cluster", apiServer: "v1.29.0", nodes: []*corev1.Node{node("a", "v1.29.0"), node("b", "v1.30.1")}, want: true},
		// Allowed version skew during a control-plane-first upgrade: a 1.28
		// kubelet rejects restartable init containers.
		{name: "1.29 API server with a 1.28 worker", apiServer: "v1.29.0", nodes: []*corev1.Node{node("a", "v1.29.0"), node("b", "v1.28.15-eks-a737599")}, want: false},
		{name: "unparsable kubelet version", apiServer: "v1.29.0", nodes: []*corev1.Node{node("a", "bogus")}, want: false},
		{name: "no nodes", apiServer: "v1.29.0", want: false},
		{name: "nodes forbidden", apiServer: "v1.29.0", nodes: []*corev1.Node{node("a", "v1.29.0")}, forbidNode: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := &fakediscovery.FakeDiscovery{
				Fake:               &clienttesting.Fake{},
				FakedServerVersion: &version.Info{GitVersion: tt.apiServer},
			}
			builder := fake.NewClientBuilder()
			for _, n := range tt.nodes {
				builder = builder.WithObjects(n)
			}
			if tt.forbidNode {
				builder = builder.WithInterceptorFuncs(interceptor.Funcs{
					List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
						return apierrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, "", errors.New("no RBAC"))
					},
				})
			}
			got, err := DetectNativeSidecars(context.Background(), sv, builder.Build())
			if err != nil {
				t.Fatalf("DetectNativeSidecars error: %v", err)
			}
			if got != tt.want {
				t.Errorf("DetectNativeSidecars = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectNativeSidecars_NodeListError(t *testing.T) {
	sv := &fakediscovery.FakeDiscovery{
		Fake:               &clienttesting.Fake{},
		FakedServerVersion: &version.Info{GitVersion: "v1.29.0"},
	}
	nodes := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return errors.New("connection refused")
		},
	}).Build()
	if _, err := DetectNativeSidecars(context.Background(), sv, nodes); err == nil {
		t.Error("expected an error when listing nodes fails")
	}
}
