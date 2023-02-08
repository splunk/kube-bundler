package subcommands

import (
	"fmt"
	"os"

	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/splunk/kube-bundler/managers"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	debug bool
	info  bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kb",
	Short: "",
	Long:  "",

	// suppress usage on failures. By default RunE prints usage on an error in a command
	// SilenceUsage disables this for all commands. Missing command or --help will still show the usage.
	SilenceUsage: true,
	// suppress duplicating error output from a command returning error and RunE itself in all subcommands.
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug output")
	rootCmd.PersistentFlags().BoolVar(&info, "info", false, "info output")
	//rootCmd.PersistentFlags().BoolP("help", "h", false, "Help message")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
	// Setup logger
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: true,
	})
	var logLevel log.Level
	if debug {
		logLevel = log.DebugLevel
	} else if info {
		logLevel = log.InfoLevel
	} else {
		logLevel = log.ErrorLevel
	}
	log.SetLevel(logLevel)
}

func setup() managers.KBClient {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		log.WithField("err", err).Fatal("couldn't get controller-runtime client")
	}
	c, err := client.New(kubeconfig, client.Options{Scheme: scheme})
	if err != nil {
		log.WithField("err", err).Fatal("couldn't create controller-runtime client")
	}

	restConfig, err := createDefaultRestConfig()
	if err != nil {
		log.WithField("err", err).Fatal("couldn't create default rest client")
	}

	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.WithField("err", err).Fatal("couldn't create client-go client")
	}

	return managers.KBClient{Client: c, Interface: cs, RestConfig: restConfig}
}

func createDefaultRestConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	defaultKubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	restConfig, err := defaultKubeconfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to initialize REST config with default kubeconfig")
	}
	return restConfig, nil
}
