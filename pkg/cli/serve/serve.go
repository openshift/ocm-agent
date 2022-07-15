package serve

import (
	"net/http"
	"strconv"

	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/openshift/ocm-agent/pkg/ocm"
	"github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	kcmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/ocm-agent/pkg/config"
	"github.com/openshift/ocm-agent/pkg/handlers"
	"github.com/openshift/ocm-agent/pkg/k8s"
	"github.com/openshift/ocm-agent/pkg/logging"
	"github.com/openshift/ocm-agent/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// serveOptions define the configuration options required by OCM agent to serve.
type serveOptions struct {
	accessToken string
	services    string
	ocmURL      string
	clusterID   string
	debug       bool
	logger      logrus.Logger
}

var (
	serviceLong = templates.LongDesc(`
	Start the OCM Agent server

	The OCM Agent would receive alerts from AlertManager and post to OCM services such as "Service Log".

	This requires an access token to be able to post to a service in OCM.
	`)

	serviceExample = templates.Examples(`
	# Start the OCM agent server
	ocm-agent serve --access-token "$TOKEN" --services "$SERVICE" --ocm-url "https://sample.example.com" --cluster-id abcd-1234

	# Start the OCM agent server by accepting token from a file (value starting with '@' is considered a file)
	ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile --cluster-id @clusteridfile

	# Start the OCM agent server in debug mode
	ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile --cluster-id @clusteridfile --debug
	`)
)

func NewServeOptions() *serveOptions {
	return &serveOptions{}
}

// NewServeCmd initializes serve command and it's flags
func NewServeCmd() *cobra.Command {
	o := NewServeOptions()
	o.logger = *logging.NewLogger()

	var cmd = &cobra.Command{
		Use:     "serve",
		Short:   "Starts the OCM Agent server",
		Long:    serviceLong,
		Example: serviceExample,
		Args:    cobra.OnlyValidArgs,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.ocmURL, config.OcmURL, "", "", "OCM URL")
	cmd.Flags().StringVarP(&o.services, config.Services, "", "", "OCM service name")
	cmd.Flags().StringVarP(&o.accessToken, config.AccessToken, "t", "", "Access token for OCM")
	cmd.Flags().StringVarP(&o.clusterID, config.ClusterID, "c", "", "Cluster ID")
	cmd.PersistentFlags().BoolVarP(&o.debug, config.Debug, "d", false, "Debug mode enable")
	kcmdutil.CheckErr(viper.BindPFlags(cmd.Flags()))

	_ = cmd.MarkFlagRequired(config.OcmURL)
	_ = cmd.MarkFlagRequired(config.Services)
	_ = cmd.MarkFlagRequired(config.AccessToken)
	_ = cmd.MarkFlagRequired(config.ClusterID)

	return cmd
}

// Complete initialisation for the server
func (o *serveOptions) Complete(cmd *cobra.Command, args []string) error {

	// ReadFlagsFromFile would read the values of flags from files (if any)
	err := ReadFlagsFromFile(cmd, config.AccessToken, config.Services, config.OcmURL, config.ClusterID)
	if err != nil {
		return err
	}

	// Check if debug mode is enabled and set the logging level accordingly
	if o.debug {
		o.logger.Level = logging.DebugLogLevel
	}

	return nil
}

func (o *serveOptions) Run() error {

	o.logger.Info("Starting ocm-agent server")
	o.logger.WithField("URL", o.ocmURL).Debug("OCM URL configured")
	o.logger.WithField("Service", o.services).Debug("OCM Service configured")

	// create new router for metrics
	rMetrics := mux.NewRouter()
	rMetrics.Path(consts.MetricsPath).Handler(promhttp.Handler())

	// Listen on the metrics port with a seprated goroutine
	o.logger.WithField("Port", consts.OCMAgentMetricsPort).Info("Start listening on metrics port")
	go func() {
		_ = http.ListenAndServe(":"+strconv.Itoa(consts.OCMAgentMetricsPort), rMetrics)
	}()

	// Initialize k8s client
	client, err := k8s.NewClient()
	if err != nil {
		o.logger.WithError(err).Fatal("Can't initialise k8s client, ensure KUBECONFIG is set")
		return err
	}

	// Initialize ocm sdk connection client
	sdkclient, err := ocm.NewConnection().Build(viper.GetString(config.OcmURL),
		viper.GetString(config.ClusterID),
		viper.GetString(config.AccessToken))
	if err != nil {
		o.logger.WithError(err).Fatal("Can't initialise OCM sdk.Connection client")
		return err
	}

	// Initialize OCMClient
	ocmclient := handlers.NewOcmClient(sdkclient)

	// create a new router
	r := mux.NewRouter()
	livezHandler := handlers.NewLivezHandler()
	readyzHandler := handlers.NewReadyzHandler()
	webhookReceiverHandler := handlers.NewWebhookReceiverHandler(client, ocmclient)
	r.Path(consts.LivezPath).Handler(livezHandler)
	r.Path(consts.ReadyzPath).Handler(readyzHandler)
	r.Path(consts.WebhookReceiverPath).Handler(webhookReceiverHandler)
	r.Use(metrics.PrometheusMiddleware)

	// serve
	o.logger.WithField("Port", consts.OCMAgentServicePort).Info("Start listening on service port")
	err = http.ListenAndServe(":"+strconv.Itoa(consts.OCMAgentServicePort), r)
	if err != nil {
		o.logger.WithError(err).Fatal("OCM Agent failed to serve")
	}

	return nil
}
