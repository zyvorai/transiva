// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/pkg/operator/controllers"
	"github.com/zyvorai/transiva/providers"
	// "github.com/zyvorai/transiva/providers/kubevirt" // TODO: Enable when KubeVirt dependencies are resolved
	"github.com/zyvorai/transiva/providers/vsphere"
)

const (
	version = "2.1.0"
)

func main() {
	// Parse flags
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file (optional, uses in-cluster config by default)")
	masterURL := flag.String("master", "", "Kubernetes master URL (optional)")
	namespace := flag.String("namespace", "", "Namespace to watch (optional, watches all namespaces by default)")
	workers := flag.Int("workers", 3, "Number of worker threads per controller")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	versionFlag := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("transiva-operator version %s\n", version)
		os.Exit(0)
	}

	// Print banner
	pterm.DefaultHeader.WithFullWidth().
		WithBackgroundStyle(pterm.NewStyle(pterm.BgDarkGray)).
		WithTextStyle(pterm.NewStyle(pterm.FgLightWhite)).
		Println("HyperSDK Kubernetes Operator")

	pterm.Info.Printfln("Version: %s", version)
	pterm.Info.Printfln("Workers per controller: %d", *workers)

	// Initialize logger
	log := logger.NewWithConfig(logger.Config{
		Level:  *logLevel,
		Format: "json",
		Output: os.Stderr,
	})

	// Build Kubernetes config
	var config *rest.Config
	var err error
	if *kubeconfig != "" {
		pterm.Info.Printfln("Using kubeconfig: %s", *kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	} else {
		pterm.Info.Println("Using in-cluster configuration")
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		pterm.Error.Printfln("Failed to build Kubernetes config: %v", err)
		os.Exit(1)
	}

	// Create Kubernetes clientset
	pterm.Info.Println("Connecting to Kubernetes...")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		pterm.Error.Printfln("Failed to create Kubernetes clientset: %v", err)
		os.Exit(1)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		pterm.Error.Printfln("Failed to connect to Kubernetes: %v", err)
		os.Exit(1)
	}

	pterm.Success.Println("Connected to Kubernetes cluster")

	// Create scheme and register types
	operatorScheme := runtime.NewScheme()
	if err := scheme.AddToScheme(operatorScheme); err != nil {
		pterm.Error.Printfln("Failed to add Kubernetes types to scheme: %v", err)
		os.Exit(1)
	}

	// TODO: Add HyperSDK types to scheme
	// if err := v1alpha1.AddToScheme(operatorScheme); err != nil {
	// 	pterm.Error.Printfln("Failed to add HyperSDK types to scheme: %v", err)
	// 	os.Exit(1)
	// }

	// Create event recorder
	// TODO: Create proper event broadcaster and recorder
	var recorder record.EventRecorder

	// Detect available export capabilities
	pterm.Info.Println("Detecting available export capabilities...")
	detector := capabilities.NewDetector(log)
	if err := detector.Detect(ctx); err != nil {
		pterm.Error.Printfln("Failed to detect capabilities: %v", err)
		os.Exit(1)
	}

	// Initialize job manager
	pterm.Info.Println("Initializing job manager...")
	jobManager := jobs.NewManager(log, detector)

	// Initialize provider registry
	pterm.Info.Println("Initializing provider registry...")
	providerRegistry := providers.NewRegistry()

	// Register providers
	providerRegistry.Register(providers.ProviderVSphere, func(cfg providers.ProviderConfig) (providers.Provider, error) {
		return vsphere.NewProvider(cfg, log)
	})

	// TODO: Enable when KubeVirt dependencies are resolved
	// providerRegistry.Register(providers.ProviderKubeVirt, func(cfg providers.ProviderConfig) (providers.Provider, error) {
	// 	return kubevirt.NewProvider(cfg, log)
	// })

	pterm.Success.Printfln("Provider registry initialized (%d providers)", len(providerRegistry.ListProviders()))

	// Create controllers
	pterm.Info.Println("Initializing controllers...")

	backupJobController := controllers.NewBackupJobController(
		clientset,
		operatorScheme,
		recorder,
		log,
		jobManager,
		providerRegistry,
	)

	backupScheduleController := controllers.NewBackupScheduleController(
		clientset,
		operatorScheme,
		recorder,
		log,
	)

	restoreJobController := controllers.NewRestoreJobController(
		clientset,
		operatorScheme,
		recorder,
		log,
		jobManager,
		providerRegistry,
	)

	pterm.Success.Println("Controllers initialized")

	// TODO: Start controller workers
	// For now, just log that we would start them
	pterm.Info.Println("Controller workers ready (not yet started - implementation pending)")
	pterm.Info.Printfln("  - BackupJob controller: %d workers", *workers)
	pterm.Info.Printfln("  - BackupSchedule controller: %d workers", *workers)
	pterm.Info.Printfln("  - RestoreJob controller: %d workers", *workers)

	// Print watch configuration
	if *namespace != "" {
		pterm.Info.Printfln("Watching namespace: %s", *namespace)
	} else {
		pterm.Info.Println("Watching all namespaces")
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	pterm.Success.Println("Operator is running")
	pterm.Info.Println("Press Ctrl+C to stop")

	// Log controller references (to satisfy Go compiler)
	_ = backupJobController
	_ = backupScheduleController
	_ = restoreJobController

	// Wait for shutdown signal
	<-sigChan

	pterm.Info.Println("\nShutdown signal received...")
	pterm.Info.Println("Stopping controllers...")

	// TODO: Stop controllers gracefully

	pterm.Success.Println("Operator stopped")
}
