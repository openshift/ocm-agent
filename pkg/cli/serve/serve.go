package serve

import (
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
	accessToken     string
	services        string
	ocmURL          string
	clusterID       string
	ocmClientID     string
	ocmClientSecret string
	debug           bool
	fleetMode       bool
	testFleetMode   bool
	logger          logrus.Logger
}

var (
	serviceLong = templates.LongDesc(`
	Start the OCM Agent server

	The OCM Agent would receive alerts from AlertManager/RHOBS and post to OCM services such as "Service Log". The ocm-agent CLI
	is able to operate in traditional OSD/ROSA mode (default) as well as in Fleet (HyperShift) mode.

	In case of traditional OSD/ROSA, the CLI requires an access token and a cluster ID to be able to post to a service in OCM and in case of fleet mode,
	it requires a client ID and a client secret to be able to authenticate with OCM.
	`)

	serviceExample = templates.Examples(`
	# Start the OCM agent server in traditional OSD/ROSA mode
	ocm-agent serve --access-token "$TOKEN" --services "$SERVICE" --ocm-url "https://sample.example.com" --cluster-id abcd-1234

	# Start the OCM agent server in traditional OSD/ROSA mode by accepting token from a file (value starting with '@' is considered a file)
	ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile --cluster-id @clusteridfile

	# Start the OCM agent server in traditional OSD/ROSA in debug mode
	ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile --cluster-id @clusteridfile --debug

	# Start OCM agent server in fleet mode
	ocm-agent serve --fleet-mode --services "$SERVICE" --ocm-url @urlfile

	# Start OCM agent server to test fleet mode (for development purposes only)
	ocm-agent serve --services $SERVICE --ocm-url $URL --fleet-mode --test-fleet-mode --ocm-client-id $CLIENT_ID --ocm-client-secret $CLIENT_SECRET
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
			testMode, _ := cmd.Flags().GetBool(config.TestFleetMode)
			if testMode {
				_ = cmd.MarkFlagRequired(config.OCMClientID)
				_ = cmd.MarkFlagRequired(config.OCMClientSecret)
				cmd.Flag(config.FleetMode)
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
	cmd.Flags().StringVarP(&o.ocmClientID, config.OCMClientID, "", "", "OCM Client ID (string)")
	cmd.Flags().StringVarP(&o.ocmClientSecret, config.OCMClientSecret, "", "", "OCM Client Secret (string)")
	cmd.Flags().BoolVar(&o.fleetMode, config.FleetMode, false, "Fleet Mode (bool)")
	cmd.Flags().BoolVar(&o.testFleetMode, config.TestFleetMode, false, "Test Fleet Mode (bool)")
	cmd.PersistentFlags().BoolVarP(&o.debug, config.Debug, "d", false, "Debug mode enable")
	kcmdutil.CheckErr(viper.BindPFlags(cmd.Flags()))

	_ = cmd.MarkFlagRequired(config.OcmURL)
	_ = cmd.MarkFlagRequired(config.Services)
	cmd.MarkFlagsRequiredTogether(config.AccessToken, config.ClusterID)
	cmd.MarkFlagsMutuallyExclusive(config.ClusterID, config.FleetMode)
	cmd.MarkFlagsMutuallyExclusive(config.TestFleetMode, config.ClusterID)

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

	var ocmAgentClientID string
	var ocmAgentClientSecret string
	var ocmAgentURL string

	o.logger.Info("Starting ocm-agent server")
	o.logger.WithField("URL", o.ocmURL).Debug("OCM URL configured")
	o.logger.WithField("Service", o.services).Debug("OCM Service configured")

	if o.fleetMode {
		o.logger.WithField("FleetMode", o.fleetMode).Info("Fleet mode configured")
	} else {
		o.logger.WithField("FleetMode", o.fleetMode).Info("Fleet mode not configured")
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
	// If fleet mode is not enabled, we will fetch the cluster ID and access token to initiate connection with OCM
	if !o.fleetMode {
		sdkclient, err = ocm.NewConnection().Build(viper.GetString(config.OcmURL),
			viper.GetString(config.ClusterID),
			viper.GetString(config.AccessToken))
		if err != nil {
			o.logger.WithError(err).Fatal("Can't initialise OCM sdk.Connection client in non-fleet mode")
			return err
		}
		o.logger.Info("Connection with OCM initialised successfully in non-fleet mode")
	} else {
		// If fleet mode is enabled, the connection to OCM needs to initiate using client ID and client secret
		// On the managed cluster, the client ID and secret will be fetched from the secret volume however for
		// local testing, the client ID and secret can be passed directly as flags for ocm-agent CLI.
		if o.testFleetMode {
			ocmAgentClientID = viper.GetString(config.OCMClientID)
			ocmAgentClientSecret = viper.GetString(config.OCMClientSecret)
			ocmAgentURL = viper.GetString(config.OcmURL)
		} else {
			clientID, err := os.ReadFile(consts.OCMAgentAccessFleetSecretPathBase + os.Getenv("OCM_AGENT_SECRET_NAME") + "/" + consts.OCMAgentAccessFleetSecretClientKey)
			ocmAgentClientID = string(clientID)
			if err != nil {
				o.logger.WithError(err).Fatal("Can't find value for secret key ", consts.OCMAgentAccessFleetSecretClientKey)
				return err
			}

			clientSecret, err := os.ReadFile(consts.OCMAgentAccessFleetSecretPathBase + os.Getenv("OCM_AGENT_SECRET_NAME") + "/" + consts.OCMAgentAccessFleetSecretClientSecretKey)
			ocmAgentClientSecret = string(clientSecret)
			if err != nil {
				o.logger.WithError(err).Fatal("Can't find value for secret key ", consts.OCMAgentAccessFleetSecretClientSecretKey)
				return err
			}

			url, err := os.ReadFile(consts.OCMAgentAccessFleetSecretPathBase + os.Getenv("OCM_AGENT_SECRET_NAME") + "/" + consts.OCMAgentAccessFleetSecretURLKey)
			ocmAgentURL = string(url)
			if err != nil {
				o.logger.WithError(err).Fatal("Can't find value for secret key ", consts.OCMAgentAccessFleetSecretURLKey)
				return err
			}
		}

		sdkclient, err = sdk.NewConnectionBuilder().URL(ocmAgentURL).Client(ocmAgentClientID, ocmAgentClientSecret).Insecure(false).Build()
		if err != nil {
			o.logger.WithError(err).Fatal("Can't initialise OCM sdk.connection client in fleet mode")
			return err
		}
		o.logger.Info("Connection with OCM initialised successfully in fleet mode")
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
