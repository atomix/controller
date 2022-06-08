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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func addBindingController(mgr manager.Manager) error {
	r := &BindingReconciler{
		BaseReconciler: &BaseReconciler{
			client: mgr.GetClient(),
			scheme: mgr.GetScheme(),
			config: mgr.GetConfig(),
		},
	}

	// Create a new controller
	c, err := controller.New("binding-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Bindings
	err = c.Watch(&source.Kind{Type: &corev3beta1.Binding{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// BindingReconciler is a Reconciler for Bindings
type BindingReconciler struct {
	*BaseReconciler
}

// Reconcile reconciles Binding resources
func (r *BindingReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Binding '%s'", request.NamespacedName)
	binding := &corev3beta1.Binding{}
	err := r.client.Get(context.TODO(), request.NamespacedName, binding)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Error(err)
		return reconcile.Result{}, err
	}

	if ok, err := r.reconcilePods(ctx, binding); ok {
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *BindingReconciler) reconcilePods(ctx context.Context, binding *corev3beta1.Binding) (bool, error) {
	pods := &corev1.PodList{}
	err := r.client.List(ctx, pods)
	if err != nil {
		return false, err
	}

	if len(pods.Items) == 0 {
		return false, nil
	}

	var returnErr error
	for _, pod := range pods.Items {
		if ok, err := r.reconcilePod(ctx, binding, &pod); ok {
			return true, returnErr
		} else if err != nil && returnErr == nil {
			returnErr = err
		}
	}
	return false, returnErr
}

func (r *BindingReconciler) reconcilePod(ctx context.Context, binding *corev3beta1.Binding, pod *corev1.Pod) (bool, error) {
	// If the pod is not controllable by this controller, skip reconciliation
	if !isControllable(pod) {
		return false, nil
	}

	log.Infof("Reconciling Binding '%s' Pod '%s'",
		types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name},
		types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name})

	conn, err := r.connect(ctx, pod)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	if ok, err := r.reconcileBinding(ctx, binding, runtimev1.NewBindingServiceClient(conn)); ok {
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}
