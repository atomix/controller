// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package v3beta1

import (
	"context"
	corev3beta1 "github.com/atomix/controller/pkg/apis/atomix/v3beta1"
	runtimev1 "github.com/atomix/runtime/api/atomix/runtime/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func addClusterController(mgr manager.Manager) error {
	r := &ClusterReconciler{
		BaseReconciler: &BaseReconciler{
			client: mgr.GetClient(),
			scheme: mgr.GetScheme(),
			config: mgr.GetConfig(),
		},
	}

	// Create a new controller
	c, err := controller.New("cluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Clusters
	err = c.Watch(&source.Kind{Type: &corev3beta1.Cluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// ClusterReconciler is a Reconciler for Clusters
type ClusterReconciler struct {
	*BaseReconciler
}

// Reconcile reconciles Cluster resources
func (r *ClusterReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Cluster '%s'", request.NamespacedName)
	cluster := &corev3beta1.Cluster{}
	err := r.client.Get(context.TODO(), request.NamespacedName, cluster)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Error(err)
		return reconcile.Result{}, err
	}

	if ok, err := r.reconcilePods(ctx, cluster); ok {
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *ClusterReconciler) reconcilePods(ctx context.Context, cluster *corev3beta1.Cluster) (bool, error) {
	pods := &corev1.PodList{}
	err := r.client.List(ctx, pods)
	if err != nil {
		return false, err
	}

	if len(pods.Items) == 0 {
		return false, nil
	}

	for _, pod := range pods.Items {
		if ok, err := r.reconcilePod(ctx, cluster, &pod); ok {
			return true, nil
		} else if err != nil {
			return false, err
		}
	}
	return false, nil
}

func (r *ClusterReconciler) reconcilePod(ctx context.Context, cluster *corev3beta1.Cluster, pod *corev1.Pod) (bool, error) {
	conn, err := r.connect(ctx, pod)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	if ok, err := r.reconcileCluster(ctx, cluster, runtimev1.NewClusterServiceClient(conn)); ok {
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}
