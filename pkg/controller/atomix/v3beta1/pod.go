// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package v3beta1

import (
	"context"
	"github.com/atomix/controller/pkg/apis/atomix/v3beta1"
	runtimev1 "github.com/atomix/runtime/api/atomix/runtime/v1"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

func addPodController(mgr manager.Manager) error {
	// Create a new controller
	options := controller.Options{
		Reconciler: &PodReconciler{
			BaseReconciler: &BaseReconciler{
				client: mgr.GetClient(),
				scheme: mgr.GetScheme(),
				config: mgr.GetConfig(),
			},
		},
		RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond*10, time.Second*5),
	}
	controller, err := controller.New("pod-controller", mgr, options)
	if err != nil {
		return err
	}

	// Watch for changes to Pods
	err = controller.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// PodReconciler is a Reconciler for Pod resources
type PodReconciler struct {
	*BaseReconciler
}

// Reconcile reconciles Pod resources
func (r *PodReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Pod '%s'", request.NamespacedName)
	pod := &corev1.Pod{}
	err := r.client.Get(ctx, request.NamespacedName, pod)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// If the runtime is not running in the pod, skip reconciliation
	if !isRuntimeEnabled(pod) {
		return reconcile.Result{}, nil
	}

	conn, err := r.connect(ctx, pod)
	if err != nil {
		return reconcile.Result{}, err
	}
	defer conn.Close()

	if ok, err := r.reconcileClusters(ctx, pod, conn); ok {
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	if ok, err := r.reconcileBindings(ctx, pod, conn); ok {
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *PodReconciler) reconcileClusters(ctx context.Context, pod *corev1.Pod, conn *grpc.ClientConn) (bool, error) {
	clusters := &v3beta1.ClusterList{}
	err := r.client.List(ctx, clusters, &client.ListOptions{Namespace: pod.Namespace})
	if err != nil {
		return false, err
	}

	if len(clusters.Items) == 0 {
		return false, nil
	}

	for _, cluster := range clusters.Items {
		log.Infof("Reconciling Pod '%s' Cluster '%s'",
			types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name},
			types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name})
		if ok, err := r.reconcileCluster(ctx, &cluster, runtimev1.NewClusterServiceClient(conn)); ok {
			return true, nil
		} else if err != nil {
			return false, err
		}
	}
	return false, nil
}

func (r *PodReconciler) reconcileBindings(ctx context.Context, pod *corev1.Pod, conn *grpc.ClientConn) (bool, error) {
	bindings := &v3beta1.BindingList{}
	err := r.client.List(ctx, bindings, &client.ListOptions{Namespace: pod.Namespace})
	if err != nil {
		return false, err
	}

	if len(bindings.Items) == 0 {
		return false, nil
	}

	for _, binding := range bindings.Items {
		log.Infof("Reconciling Pod '%s' Binding '%s'",
			types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name},
			types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name})
		if ok, err := r.reconcileBinding(ctx, &binding, runtimev1.NewBindingServiceClient(conn)); ok {
			return true, nil
		} else if err != nil {
			return false, err
		}
	}
	return false, nil
}

func getControlPort(pod *corev1.Pod) int {
	for _, container := range pod.Spec.Containers {
		if container.Name == runtimeContainerName {
			for _, port := range container.Ports {
				if port.Name == controlPortName {
					return int(port.ContainerPort)
				}
			}
		}
	}
	return defaultControlPort
}
