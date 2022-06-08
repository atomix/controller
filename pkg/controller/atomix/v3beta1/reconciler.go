// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package v3beta1

import (
	"context"
	"fmt"
	"github.com/atomix/controller/pkg/apis/atomix/v3beta1"
	runtimev1 "github.com/atomix/runtime/api/atomix/runtime/v1"
	"github.com/atomix/runtime/pkg/atomix/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BaseReconciler is a Reconciler for Pod resources
type BaseReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

func (r *BaseReconciler) connect(ctx context.Context, pod *corev1.Pod) (*grpc.ClientConn, error) {
	controlPort := getControlPort(pod)
	target := fmt.Sprintf("%s:%d", pod.Status.PodIP, controlPort)
	return grpc.DialContext(ctx, target, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func (r *BaseReconciler) reconcileCluster(ctx context.Context, cluster *v3beta1.Cluster, client runtimev1.ClusterServiceClient) (bool, error) {
	getRequest := &runtimev1.GetClusterRequest{
		ClusterID: runtimev1.ClusterId{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		},
	}
	_, err := client.GetCluster(ctx, getRequest)
	if err == nil {
		return false, nil
	}
	err = errors.FromProto(err)
	if !errors.IsNotFound(err) {
		return false, err
	}

	createRequest := &runtimev1.CreateClusterRequest{
		Cluster: &runtimev1.Cluster{
			ClusterMeta: runtimev1.ClusterMeta{
				ID: runtimev1.ClusterId{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
				},
			},
			Spec: runtimev1.ClusterSpec{
				Driver: runtimev1.DriverId{
					Name:    cluster.Spec.Driver.Name,
					Version: cluster.Spec.Driver.Version,
				},
				Config: cluster.Spec.Config.Raw,
			},
		},
	}
	_, err = client.CreateCluster(ctx, createRequest)
	if err == nil {
		return false, nil
	}
	err = errors.FromProto(err)
	if !errors.IsAlreadyExists(err) {
		return false, err
	}
	return true, nil
}

func (r *BaseReconciler) reconcileBinding(ctx context.Context, binding *v3beta1.Binding, client runtimev1.BindingServiceClient) (bool, error) {
	getRequest := &runtimev1.GetBindingRequest{
		BindingID: runtimev1.BindingId{
			Namespace: binding.Namespace,
			Name:      binding.Name,
		},
	}
	_, err := client.GetBinding(ctx, getRequest)
	if err == nil {
		return false, nil
	}
	err = errors.FromProto(err)
	if !errors.IsNotFound(err) {
		return false, err
	}

	clusterNamespace := binding.Spec.Cluster.Namespace
	if clusterNamespace == "" {
		clusterNamespace = binding.Namespace
	}

	rules := make([]runtimev1.BindingRule, len(binding.Spec.Rules))
	for i, rule := range binding.Spec.Rules {
		rules[i] = runtimev1.BindingRule{
			Kinds:    rule.Kinds,
			Names:    rule.Names,
			Metadata: rule.Metadata,
		}
	}

	createRequest := &runtimev1.CreateBindingRequest{
		Binding: &runtimev1.Binding{
			BindingMeta: runtimev1.BindingMeta{
				ID: runtimev1.BindingId{
					Namespace: binding.Namespace,
					Name:      binding.Name,
				},
			},
			Spec: runtimev1.BindingSpec{
				ClusterID: runtimev1.ClusterId{
					Namespace: clusterNamespace,
					Name:      binding.Spec.Cluster.Name,
				},
				Rules: rules,
			},
		},
	}
	_, err = client.CreateBinding(ctx, createRequest)
	if err == nil {
		return false, nil
	}
	err = errors.FromProto(err)
	if !errors.IsAlreadyExists(err) {
		return false, err
	}
	return true, nil
}

func isControllable(pod *corev1.Pod) bool {
	return pod.Annotations[runtimeInjectStatusAnnotation] == injectedStatus &&
		pod.Annotations[runtimeVersionAnnotation] == os.Getenv(runtimeVersionEnv)
}
