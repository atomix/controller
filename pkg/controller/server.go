// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	controllerv1 "github.com/atomix/controller/api/atomix/controller/v1"
	atomixv1beta1 "github.com/atomix/controller/pkg/apis/atomix/v1beta1"
	"github.com/atomix/runtime/pkg/atomix/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const wildcard = "*"

func NewControllerServer(client client.Client) controllerv1.ControllerServer {
	return &Server{
		client: client,
	}
}

type Server struct {
	client client.Client
}

func (s *Server) OpenSession(ctx context.Context, request *controllerv1.OpenSessionRequest) (*controllerv1.OpenSessionResponse, error) {
	profile := &atomixv1beta1.Profile{}
	profileNamespacedName := types.NamespacedName{
		Namespace: request.Session.Namespace,
		Name:      request.Session.ProfileName,
	}
	if err := s.client.Get(ctx, profileNamespacedName, profile); err != nil {
		return nil, errors.ToProto(errors.NewInternal(err.Error()))
	}

	for i, podStatus := range profile.Status.PodStatuses {
		if string(podStatus.UID) == request.Session.PodID {
			for _, sessionStatus := range podStatus.Sessions {
				if string(sessionStatus.UID) == request.Session.SessionID {
					response := &controllerv1.OpenSessionResponse{}
					return response, nil
				}
			}

			metadata := make(map[string][]string)
			for key, values := range request.Session.Metadata.Metadata {
				metadata[key] = values.Values
			}

			podStatus.Sessions = append(podStatus.Sessions, atomixv1beta1.ProfileSessionStatus{
				ObjectReference: corev1.ObjectReference{
					Name: request.Session.PrimitiveName,
					UID:  types.UID(request.Session.SessionID),
				},
				Service:           request.Session.ServiceName,
				Metadata:          metadata,
				CreationTimestamp: metav1.Now(),
				State:             atomixv1beta1.ProfileSessionUnbound,
			})
			profile.Status.PodStatuses[i] = podStatus

			if err := s.client.Update(ctx, profile); err != nil {
				return nil, errors.ToProto(errors.NewInternal(err.Error()))
			}
			response := &controllerv1.OpenSessionResponse{}
			return response, nil
		}
	}
	return nil, errors.ToProto(errors.NewUnavailable("pod '%s' status not found for Profile '%s'", request.Session.PodID, profileNamespacedName))
}

func (s *Server) CloseSession(ctx context.Context, request *controllerv1.CloseSessionRequest) (*controllerv1.CloseSessionResponse, error) {
	profile := &atomixv1beta1.Profile{}
	profileNamespacedName := types.NamespacedName{
		Namespace: request.Session.Namespace,
		Name:      request.Session.ProfileName,
	}
	if err := s.client.Get(ctx, profileNamespacedName, profile); err != nil {
		return nil, errors.ToProto(errors.NewInternal(err.Error()))
	}

	for i, podStatus := range profile.Status.PodStatuses {
		for j, sessionStatus := range podStatus.Sessions {
			if sessionStatus.UID == types.UID(request.Session.SessionID) {
				now := metav1.Now()
				sessionStatus.DeletionTimestamp = &now
				podStatus.Sessions[j] = sessionStatus
			}
		}
		profile.Status.PodStatuses[i] = podStatus

		if err := s.client.Update(ctx, profile); err != nil {
			return nil, errors.ToProto(errors.NewInternal(err.Error()))
		}
		response := &controllerv1.CloseSessionResponse{}
		return response, nil
	}
	return nil, errors.ToProto(errors.NewUnavailable("pod '%s' status not found for Profile '%s'", request.Session.PodID, profileNamespacedName))
}

var _ controllerv1.ControllerServer
