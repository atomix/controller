// SPDX-FileCopyrightText: 2022-present Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	controllerv1 "github.com/atomix/controller/api/atomix/controller/v1"
	"github.com/atomix/controller/pkg/apis"
	"github.com/atomix/controller/pkg/controller"
	corev1beta1 "github.com/atomix/controller/pkg/controller/atomix/v1beta1"
	"github.com/atomix/controller/pkg/controller/util/k8s"
	"github.com/atomix/runtime/pkg/atomix/logging"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"net"
	"os"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

const defaultPort = 5680

var log = logging.GetLogger()

func init() {
	logging.SetLevel(logging.DebugLevel)
}

func main() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	logf.SetLogger(logr.New(&ControllerLogSink{log}))

	cmd := getCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "atomix-controller",
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			namespace, _ := cmd.Flags().GetString("namespace")
			port, _ := cmd.Flags().GetInt("port")

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

			server := grpc.NewServer()
			lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}

			controllerv1.RegisterControllerServer(server, controller.NewControllerServer(mgr.GetClient()))

			go func() {
				log.Info("Starting the Server")
				if err := server.Serve(lis); err != nil {
					log.Error(err)
					os.Exit(1)
				}
			}()

			// Start the manager
			log.Info("Starting the Manager")
			mgr.GetWebhookServer().Port = 443
			if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
				log.Error(err, "controller exited non-zero")
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringP("namespace", "n", "", "the namespace in which to run the controller")
	cmd.Flags().IntP("port", "p", defaultPort, "the port on which to run the controller server")
	return cmd
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
