// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package v3beta1

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strconv"
)

const (
	nodeEnv      = "ATOMIX_NODE"
	namespaceEnv = "ATOMIX_NAMESPACE"
	nameEnv      = "ATOMIX_NAME"
)

const (
	runtimeInjectPath             = "/inject-runtime"
	runtimeInjectAnnotation       = "runtime.atomix.io/inject"
	runtimeInjectStatusAnnotation = "runtime.atomix.io/status"
	atomixReadyCondition          = "AtomixReady"
	injectedStatus                = "injected"
)

const (
	defaultRuntimeImageEnv = "DEFAULT_RUNTIME_IMAGE"
	defaultRuntimeImage    = "atomix/atomix-runtime:latest"
)

func getDefaultRuntimeImage() string {
	image := os.Getenv(defaultRuntimeImageEnv)
	if image == "" {
		image = defaultRuntimeImage
	}
	return image
}

func addRuntimeController(mgr manager.Manager) error {
	mgr.GetWebhookServer().Register(runtimeInjectPath, &webhook.Admission{
		Handler: &RuntimeInjector{
			client: mgr.GetClient(),
			scheme: mgr.GetScheme(),
		},
	})
	return nil
}

// RuntimeInjector is a mutating webhook that injects the runtime container into pods
type RuntimeInjector struct {
	client  client.Client
	scheme  *runtime.Scheme
	decoder *admission.Decoder
}

// InjectDecoder :
func (i *RuntimeInjector) InjectDecoder(decoder *admission.Decoder) error {
	i.decoder = decoder
	return nil
}

// Handle :
func (i *RuntimeInjector) Handle(ctx context.Context, request admission.Request) admission.Response {
	podNamespacedName := types.NamespacedName{
		Namespace: request.Namespace,
		Name:      request.Name,
	}
	log.Infof("Received admission request for Pod '%s'", podNamespacedName)

	// Decode the pod
	pod := &corev1.Pod{}
	if err := i.decoder.Decode(request, pod); err != nil {
		log.Errorf("Could not decode Pod '%s'", podNamespacedName, err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	injectRuntime, ok := pod.Annotations[runtimeInjectAnnotation]
	if !ok {
		log.Infof("Skipping runtime injection for Pod '%s'", podNamespacedName)
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", runtimeInjectAnnotation))
	}
	if inject, err := strconv.ParseBool(injectRuntime); err != nil {
		log.Errorf("Runtime injection failed for Pod '%s'", podNamespacedName, err)
		return admission.Allowed(fmt.Sprintf("'%s' annotation could not be parsed", runtimeInjectAnnotation))
	} else if !inject {
		log.Infof("Skipping runtime injection for Pod '%s'", podNamespacedName)
		return admission.Allowed(fmt.Sprintf("'%s' annotation is false", runtimeInjectAnnotation))
	}

	injectedRuntime, ok := pod.Annotations[runtimeInjectStatusAnnotation]
	if ok && injectedRuntime == injectedStatus {
		log.Infof("Skipping runtime injection for Pod '%s'", podNamespacedName)
		return admission.Allowed(fmt.Sprintf("'%s' annotation is '%s'", runtimeInjectStatusAnnotation, injectedRuntime))
	}

	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:            "atomix-runtime",
		Image:           getDefaultRuntimeImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			{
				Name: namespaceEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: nameEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: nodeEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
		},
	})
	pod.Spec.ReadinessGates = append(pod.Spec.ReadinessGates, corev1.PodReadinessGate{
		ConditionType: atomixReadyCondition,
	})
	pod.Annotations[runtimeInjectStatusAnnotation] = injectedStatus

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		log.Errorf("Runtime injection failed for Pod '%s'", podNamespacedName, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

var _ admission.Handler = &RuntimeInjector{}
