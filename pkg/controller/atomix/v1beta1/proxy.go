// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"context"
	"fmt"
	atomixv1beta1 "github.com/atomix/controller/pkg/apis/atomix/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strconv"
	"strings"
)

const (
	podIDEnv        = "POD_ID"
	podNamespaceEnv = "POD_NAMESPACE"
	podNameEnv      = "POD_NAME"
	nodeIDEnv       = "NODE_ID"
	profileNameEnv  = "PROFILE_NAME"
)

const (
	proxyInjectPath               = "/inject-proxy"
	proxyInjectAnnotation         = "proxy.atomix.io/inject"
	proxyInjectStatusAnnotation   = "proxy.atomix.io/status"
	proxyRuntimeVersionAnnotation = "proxy.atomix.io/runtime-version"
	proxyDriversAnnotation        = "proxy.atomix.io/drivers"
	proxyProfileAnnotation        = "proxy.atomix.io/profile"
	injectedStatus                = "injected"
	proxyContainerName            = "atomix-proxy"
)

const (
	runtimeVersionEnv = "ATOMIX_RUNTIME_VERSION"
	proxyImageEnv     = "ATOMIX_PROXY_IMAGE"
	defaultProxyImage = "atomix/proxy:latest"
)

func getProxyImage() string {
	image := os.Getenv(proxyImageEnv)
	if image != "" {
		return image
	}
	return defaultProxyImage
}

func addProxyController(mgr manager.Manager) error {
	mgr.GetWebhookServer().Register(proxyInjectPath, &webhook.Admission{
		Handler: &RuntimeInjector{
			client: mgr.GetClient(),
			scheme: mgr.GetScheme(),
		},
	})
	return nil
}

// RuntimeInjector is a mutating webhook that injects the proxy container into pods
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

	injectRuntime, ok := pod.Annotations[proxyInjectAnnotation]
	if !ok {
		log.Infof("Skipping proxy injection for Pod '%s'", podNamespacedName)
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", proxyInjectAnnotation))
	}
	if inject, err := strconv.ParseBool(injectRuntime); err != nil {
		log.Errorf("Runtime injection failed for Pod '%s'", podNamespacedName, err)
		return admission.Allowed(fmt.Sprintf("'%s' annotation could not be parsed", proxyInjectAnnotation))
	} else if !inject {
		log.Infof("Skipping proxy injection for Pod '%s'", podNamespacedName)
		return admission.Allowed(fmt.Sprintf("'%s' annotation is false", proxyInjectAnnotation))
	}

	injectedRuntime, ok := pod.Annotations[proxyInjectStatusAnnotation]
	if ok && injectedRuntime == injectedStatus {
		log.Infof("Skipping proxy injection for Pod '%s'", podNamespacedName)
		return admission.Allowed(fmt.Sprintf("'%s' annotation is '%s'", proxyInjectStatusAnnotation, injectedRuntime))
	}

	var proxyArgs []string

	var driverNames []string
	runtimeVersion := os.Getenv(runtimeVersionEnv)

	profileName, ok := pod.Annotations[proxyProfileAnnotation]
	if !ok {
		log.Warnf("No profile specified for Pod '%s'", podNamespacedName)
	} else {
		profile := &atomixv1beta1.Profile{}
		profileNamespacedName := types.NamespacedName{
			Namespace: request.Namespace,
			Name:      profileName,
		}
		if err := i.client.Get(ctx, profileNamespacedName, profile); err != nil {
			log.Errorf("Runtime injection failed for Pod '%s'", podNamespacedName, err)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		for _, binding := range profile.Spec.Bindings {
			store := &atomixv1beta1.Store{}
			storeNamespace := binding.Store.Namespace
			if storeNamespace == "" {
				storeNamespace = request.Namespace
			}
			storeNamespacedName := types.NamespacedName{
				Namespace: storeNamespace,
				Name:      binding.Store.Name,
			}
			if err := i.client.Get(ctx, storeNamespacedName, store); err != nil {
				log.Errorf("Runtime injection failed for Pod '%s'", podNamespacedName, err)
				return admission.Errored(http.StatusInternalServerError, err)
			}

			protocol := &atomixv1beta1.Protocol{}
			protocolNamespacedName := types.NamespacedName{
				Name: store.Spec.Protocol.Name,
			}
			if err := i.client.Get(ctx, protocolNamespacedName, protocol); err != nil {
				log.Errorf("Runtime injection failed for Pod '%s'", podNamespacedName, err)
				return admission.Errored(http.StatusInternalServerError, err)
			}

			var protocolVersion *atomixv1beta1.ProtocolVersion
			for _, version := range protocol.Versions {
				if version.Name == store.Spec.Protocol.Version {
					protocolVersion = &version
					break
				}
			}

			if protocolVersion == nil {
				log.Infof("Skipping runtime injection for Pod '%s'", podNamespacedName)
				return admission.Denied(fmt.Sprintf("Unknown version '%s' for protocol '%s'", store.Spec.Protocol.Version, store.Spec.Protocol.Name))
			}

			var protocolDriver *atomixv1beta1.ProtocolDriver
			for _, driver := range protocolVersion.Drivers {
				if driver.RuntimeVersion == runtimeVersion {
					protocolDriver = &driver
					break
				}
			}

			if protocolDriver == nil {
				log.Infof("Skipping runtime injection for Pod '%s'", podNamespacedName)
				return admission.Denied(fmt.Sprintf("Unknown runtime version '%s' for protocol '%s'", runtimeVersion, store.Spec.Protocol.Name))
			}

			log.Infof("Injecting Protocol '%s' driver version '%s' into Pod '%s'", protocol.Name, protocolVersion.Name, podNamespacedName)
			protocolName := fmt.Sprintf("%s-%s", protocol.Name, protocolVersion.Name)
			driverFile := fmt.Sprintf("%s.so", protocolName)
			pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
				Name:  protocolName,
				Image: protocolDriver.Image,
				Command: []string{
					"cp",
					protocolDriver.Path,
					filepath.Join("/drivers", driverFile),
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "drivers",
						MountPath: "/drivers",
						SubPath:   protocolName,
					},
				},
			})

			proxyArgs = append(proxyArgs, "--driver", filepath.Join("/var/atomix/drivers", protocolVersion.Name))
		}
	}

	proxyArgs = append(proxyArgs, "--config", fmt.Sprintf("/etc/atomix/%s", configFile))

	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:            proxyContainerName,
		Image:           getProxyImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args:            proxyArgs,
		Env: []corev1.EnvVar{
			{
				Name: podIDEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.podID",
					},
				},
			},
			{
				Name: podNamespaceEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: podNameEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: nodeIDEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
			{
				Name:  profileNameEnv,
				Value: profileName,
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "runtime",
				ContainerPort: 5678,
			},
			{
				Name:          "control",
				ContainerPort: 5679,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config",
				ReadOnly:  true,
				MountPath: "/etc/atomix",
			},
			{
				Name:      "drivers",
				ReadOnly:  true,
				MountPath: "/var/atomix/drivers",
			},
		},
	})
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: profileName,
				},
			},
		},
	})
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "drivers",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	pod.Annotations[proxyInjectStatusAnnotation] = injectedStatus
	pod.Annotations[proxyRuntimeVersionAnnotation] = runtimeVersion
	pod.Annotations[proxyDriversAnnotation] = strings.Join(driverNames, ",")

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		log.Errorf("Runtime injection failed for Pod '%s'", podNamespacedName, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

var _ admission.Handler = &RuntimeInjector{}
