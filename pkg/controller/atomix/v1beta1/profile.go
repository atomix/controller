// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"context"
	atomixv1beta1 "github.com/atomix/controller/pkg/apis/atomix/v1beta1"
	"github.com/atomix/proxy/pkg/proxy"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const configFile = "config.yaml"

func addProfileController(mgr manager.Manager) error {
	r := &ProfileReconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
	}

	// Create a new controller
	c, err := controller.New("profile-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Profiles
	err = c.Watch(&source.Kind{Type: &atomixv1beta1.Profile{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to Pods
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
		profileName, ok := object.GetAnnotations()[proxyProfileAnnotation]
		if !ok {
			return nil
		}
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: object.GetNamespace(),
					Name:      profileName,
				},
			},
		}
	}))

	// Watch for changes to Stores
	err = c.Watch(&source.Kind{Type: &atomixv1beta1.Store{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
		profiles := &atomixv1beta1.ProfileList{}
		if err := mgr.GetClient().List(context.Background(), profiles, nil); err != nil {
			return nil
		}

		var requests []reconcile.Request
		for _, profile := range profiles.Items {
			for _, binding := range profile.Spec.Bindings {
				storeNamespace := binding.Store.Namespace
				if storeNamespace == "" {
					storeNamespace = profile.Namespace
				}
				if storeNamespace == object.GetNamespace() && binding.Store.Name == object.GetName() {
					requests = append(requests, reconcile.Request{
						NamespacedName: getNamespacedName(&profile),
					})
				}
			}
		}
		return requests
	}))
	return nil
}

// ProfileReconciler is a Reconciler for Profiles
type ProfileReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// Reconcile reconciles Profile resources
func (r *ProfileReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Profile '%s'", request.NamespacedName)
	profile := &atomixv1beta1.Profile{}
	err := r.client.Get(context.TODO(), request.NamespacedName, profile)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Error(err)
		return reconcile.Result{}, err
	}

	configMap := &corev1.ConfigMap{}
	if err := r.client.Get(ctx, getNamespacedName(profile), configMap); err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Error(err)
			return reconcile.Result{}, err
		}

		var routerConfig proxy.RouterConfig
		for _, binding := range profile.Spec.Bindings {
			var route proxy.RouteConfig
			storeNamespace := binding.Store.Namespace
			if storeNamespace == "" {
				storeNamespace = profile.Namespace
			}
			route.Store = proxy.StoreConfig{
				Namespace: storeNamespace,
				Name:      binding.Store.Name,
			}
			for _, primitive := range binding.Primitives {
				rule := proxy.RuleConfig{
					Kinds:       primitive.Kinds,
					APIVersions: primitive.APIVersions,
					Names:       primitive.Names,
					Metadata:    primitive.Metadata,
				}
				route.Rules = append(route.Rules, rule)
			}
			routerConfig.Routes = append(routerConfig.Routes, route)
		}

		configBytes, err := yaml.Marshal(routerConfig)
		if err != nil {
			log.Error(err)
			return reconcile.Result{}, err
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: profile.Namespace,
				Name:      profile.Name,
			},
			BinaryData: map[string][]byte{
				configFile: configBytes,
			},
		}

		if err := controllerutil.SetOwnerReference(profile, configMap, r.scheme); err != nil {
			log.Error(err)
			return reconcile.Result{}, err
		}

		if err := r.client.Create(ctx, configMap); err != nil {
			log.Error(err)
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}
