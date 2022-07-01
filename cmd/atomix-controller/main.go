// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"github.com/atomix/controller/pkg/apis"
	corev1beta1 "github.com/atomix/controller/pkg/controller/atomix/v1beta1"
	"github.com/atomix/controller/pkg/controller/util/k8s"
	"github.com/atomix/runtime/pkg/atomix/logging"
	"github.com/go-logr/logr"
	"os"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var log = logging.GetLogger()

func init() {
	logging.SetLevel(logging.DebugLevel)
}

func main() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	logf.SetLogger(logr.New(&ControllerLogSink{log}))

	var namespace string
	if len(os.Args) > 1 {
		namespace = os.Args[1]
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Become the leader before proceeding
	_ = k8s.BecomeLeader(context.TODO())

	r := k8s.NewFileReady()
	err = r.Set()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	defer func() {
		_ = r.Unset()
	}()

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{Namespace: namespace})
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Add all the controllers
	if err := corev1beta1.AddControllers(mgr); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Start the manager
	log.Info("Starting the Manager")
	mgr.GetWebhookServer().Port = 443
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "controller exited non-zero")
		os.Exit(1)
	}
}

type ControllerLogSink struct {
	log logging.Logger
}

func (l *ControllerLogSink) Init(info logr.RuntimeInfo) {

}

func (l *ControllerLogSink) Enabled(level int) bool {
	return true
}

func (l *ControllerLogSink) Info(level int, msg string, keysAndValues ...interface{}) {
	l.log.WithFields(getFields(keysAndValues...)...).Info(msg)
}

func (l *ControllerLogSink) Error(err error, msg string, keysAndValues ...interface{}) {
	l.log.WithFields(getFields(keysAndValues...)...).Error(err, msg)
}

func (l *ControllerLogSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return &ControllerLogSink{
		log: l.log.WithFields(getFields(keysAndValues...)...),
	}
}

func (l *ControllerLogSink) WithName(name string) logr.LogSink {
	return &ControllerLogSink{
		log: l.log.GetLogger(name),
	}
}

var _ logr.LogSink = (*ControllerLogSink)(nil)

func getFields(keysAndValues ...interface{}) []logging.Field {
	fields := make([]logging.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key := keysAndValues[i]
		value := keysAndValues[i+1]
		field := logging.String(fmt.Sprint(key), fmt.Sprint(value))
		fields = append(fields, field)
	}
	return fields
}
