// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"context"
	"fmt"
	atomixv1beta1 "github.com/atomix/controller/pkg/apis/atomix/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const storeValidatePath = "/validate-store"

func addStoreController(mgr manager.Manager) error {
	mgr.GetWebhookServer().Register(storeValidatePath, &webhook.Admission{
		Handler: &StoreValidator{
			client: mgr.GetClient(),
			scheme: mgr.GetScheme(),
		},
	})
	return nil
}

// StoreValidator is a mutating webhook that injects the proxy container into pods
type StoreValidator struct {
	client  client.Client
	scheme  *runtime.Scheme
	decoder *admission.Decoder
}

// InjectDecoder :
func (i *StoreValidator) InjectDecoder(decoder *admission.Decoder) error {
	i.decoder = decoder
	return nil
}

// Handle :
func (i *StoreValidator) Handle(ctx context.Context, request admission.Request) admission.Response {
	storeNamespacedName := types.NamespacedName{
		Namespace: request.Namespace,
		Name:      request.Name,
	}
	log.Infof("Received admission request for Store '%s'", storeNamespacedName)

	// Decode the pod
	store := &atomixv1beta1.Store{}
	if err := i.decoder.Decode(request, store); err != nil {
		log.Errorf("Could not decode Store '%s'", storeNamespacedName, err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	protocolNamespacedName := types.NamespacedName{
		Name: store.Spec.Protocol.Name,
	}
	protocol := &atomixv1beta1.Protocol{}
	if err := i.client.Get(ctx, protocolNamespacedName, protocol); err != nil {
		if errors.IsNotFound(err) {
			return admission.Denied(fmt.Sprintf("protocol '%s' not found", protocolNamespacedName))
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	var version *atomixv1beta1.ProtocolVersion
	for _, v := range protocol.Spec.Versions {
		if v.Name == store.Spec.Protocol.Version {
			version = &v
			break
		}
	}

	if version == nil {
		return admission.Denied(fmt.Sprintf("protocol '%s' does not support version %s", protocolNamespacedName, store.Spec.Protocol.Version))
	}
	return admission.Allowed("")
}

var _ admission.Handler = &StoreValidator{}
