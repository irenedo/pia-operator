package main

import (
	"context"
	"flag"
	"os"

	"github.com/irenedo/pia-operator/internal/controller"

	"github.com/irenedo/pia-operator/pkg/awsclient"
	"github.com/irenedo/pia-operator/pkg/k8sclient"
	metrics "github.com/irenedo/pia-operator/pkg/metrics"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	ctx := context.Background()
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var awsRegion string
	var clusterName string
	var devMode bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&awsRegion, "aws-region", "", "AWS region for EKS operations")
	flag.StringVar(&clusterName, "cluster-name", "", "EKS cluster name")
	flag.BoolVar(&devMode, "dev-mode", false, "Enable development logging mode (more verbose logs)")

	opts := zap.Options{
		Development: devMode,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if awsRegion == "" {
		awsRegion = "eu-west-1"
	}

	if clusterName == "" {
		setupLog.Error(nil, "cluster-name flag is required")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "pia-operator.eks.aws.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	// Register custom metrics
	metrics.RegisterMetrics(ctrlmetrics.Registry)

	awsClient, err := awsclient.NewClient(ctx, clusterName, awsRegion, ctrl.Log.WithName("controllers").WithName("ServiceAccount"))
	if err != nil {
		setupLog.Error(err, "unable to create AWS client")
		os.Exit(1)
	}

	reconciler := &controller.ServiceAccountReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		Log:         ctrl.Log.WithName("controllers").WithName("ServiceAccount"),
		AWSRegion:   awsRegion,
		ClusterName: clusterName,
		AWSClient:   awsClient,
		K8sClient:   k8sclient.NewClient(mgr.GetClient()),
	}

	if err = reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceAccount")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
