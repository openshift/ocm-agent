package serve

import (
	"fmt"
	"net/http"
	"os"
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

	sdk "github.com/openshift-online/ocm-sdk-go"
)

// serveOptions define the configuration options required by OCM agent to serve.
type serveOptions struct {
	accessToken string
	services    string
	ocmURL      string
	clusterID   string
	debug       bool
	fleetMode   bool
	logger      logrus.Logger
}

var (
	serviceLong = templates.LongDesc(`
	Start the OCM Agent server

	The OCM Agent would receive alerts from AlertManager/RHOBS and post to OCM services such as "Service Log". The ocm-agent CLI
	is able to operate in traditional OSD/ROSA mode (default) as well as in Fleet (HyperShift) mode.

	In case of traditional OSD/ROSA, this requires an access token to be able to post to a service in OCM and in case of HyperShift
	mode, it requires a serviceaccount to be mounted to be able to authenticate with OCM.
	`)

	serviceExample = templates.Examples(`
	# Start the OCM agent server in traditional OSD/ROSA mode
	ocm-agent serve --access-token "$TOKEN" --services "$SERVICE" --ocm-url "https://sample.example.com" --cluster-id abcd-1234

	# Start the OCM agent server in traditional OSD/ROSA mode by accepting token from a file (value starting with '@' is considered a file)
	ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile --cluster-id @clusteridfile

	# Start the OCM agent server in traditional OSD/ROSA in debug mode
	ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile --cluster-id @clusteridfile --debug

	# Start OCM agent server in Fleet mode
	ocm-agent serve --fleet-mode --services "$SERVICE" --ocm-url @urlfile
	`)

	sdkclient *sdk.Connection
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
		PreRun: func(cmd *cobra.Command, args []string) {
			mode, _ := cmd.Flags().GetBool(config.FleetMode)
			if !mode {
				_ = cmd.MarkFlagRequired(config.AccessToken)
				_ = cmd.MarkFlagRequired(config.ClusterID)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.ocmURL, config.OcmURL, "", "", "OCM URL (string)")
	cmd.Flags().StringVarP(&o.services, config.Services, "", "", "OCM service name (string)")
	cmd.Flags().StringVarP(&o.accessToken, config.AccessToken, "t", "", "Access token for OCM (string)")
	cmd.Flags().StringVarP(&o.clusterID, config.ClusterID, "c", "", "Cluster ID (string)")
	cmd.Flags().BoolVar(&o.fleetMode, config.FleetMode, false, "Fleet Mode (bool)")
	cmd.PersistentFlags().BoolVarP(&o.debug, config.Debug, "d", false, "Debug mode enable")
	kcmdutil.CheckErr(viper.BindPFlags(cmd.Flags()))

	_ = cmd.MarkFlagRequired(config.OcmURL)
	_ = cmd.MarkFlagRequired(config.Services)
	cmd.MarkFlagsRequiredTogether(config.AccessToken, config.ClusterID)
	cmd.MarkFlagsMutuallyExclusive(config.ClusterID, config.FleetMode)

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

	if o.fleetMode {
		o.logger.WithField("FleetMode", o.fleetMode).Debug("Fleet mode configured")
	} else {
		o.logger.WithField("FleetMode", o.fleetMode).Debug("Fleet mode not configured")
	}

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

	// Depending on whether the FleetMode is enabled or not, we need to initiate the OCM SDK connection accordingly
	if !o.fleetMode {
		sdkclient, err = ocm.NewConnection().Build(viper.GetString(config.OcmURL),
			viper.GetString(config.ClusterID),
			viper.GetString(config.AccessToken))
		if err != nil {
			o.logger.WithError(err).Fatal("Can't initialise OCM sdk.Connection client in non-fleet mode")
			return err
		}
	} else {
		ocmAgentClientID, hasOcmAgentClientID := os.LookupEnv("OA_OCM_CLIENT_ID")
		ocmAgentClientSecret, hasOcmAgentClientSecret := os.LookupEnv("OA_OCM_CLIENT_SECRET")
		ocmAgentURL, hasOcmURL := os.LookupEnv("OA_OCM_URL")
		if !hasOcmAgentClientID || !hasOcmAgentClientSecret || !hasOcmURL {
			_ = fmt.Errorf("missing environment variables: OA_OCM_CLIENT_ID OA_OCM_CLIENT_SECRET OA_OCM_URL")
		}
		sdkclient, err = sdk.NewConnectionBuilder().URL(ocmAgentURL).Client(ocmAgentClientID, ocmAgentClientSecret).Insecure(false).Build()
		if err != nil {
			o.logger.WithError(err).Fatal("Can't initialise OCM sdk.connection client in fleet mode")
		}
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
