package serve

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/ocm-agent/pkg/webhookreceiver"
)

type level log.Level

func (l *level) String() string {
	return log.Level(*l).String()
}

func (l *level) Set(value string) error {
	lvl, err := log.ParseLevel(strings.TrimSpace(value))
	if err == nil {
		*l = level(lvl)
	}
	return err
}

var (
	defaultLogLevel = log.InfoLevel.String()
	logLevel        level
)

// serveOptions define the configuration options required by OCM agent to serve.
type serveOptions struct {
	accessToken string
	services    string
	ocmURL      string
	debug       bool
}

var (
	serviceLong = templates.LongDesc(`
	Start the OCM Agent server

	The OCM Agent would receive alerts from AlertManager and post to OCM services such as "Service Log".

	This requires an access token to be able to post to a service in OCM.
	`)

	serviceExample = templates.Examples(`
	# Start the OCM agent server
	ocm-agent serve --access-token "$TOKEN" --services "$SERVICE" --ocm-url "https://sample.example.com"

	# Start the OCM agent server by accepting token from a file (value starting with '@' is considered a file)
	ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile

	# Start the OCM agent server in debug mode
	ocm-agent serve -t @tokenfile --services "$SERVICE" --ocm-url @urlfile --debug
	`)
)

const (
	port                = 8081
	accessTokenFlagName = "access-token"
	servicesFlagName    = "services"
	ocmURLFlagName      = "ocm-url"
	debugFlagName       = "debug"
)

func NewServeOptions() *serveOptions {
	return &serveOptions{}
}

// NewServeCmd initializes serve command and it's flags
func NewServeCmd() *cobra.Command {
	o := NewServeOptions()
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

	cmd.Flags().StringVarP(&o.ocmURL, ocmURLFlagName, "", "", "OCM URL")
	cmd.Flags().StringVarP(&o.services, servicesFlagName, "", "", "OCM service name")
	cmd.Flags().StringVarP(&o.accessToken, accessTokenFlagName, "t", "", "Access token for OCM")
	cmd.PersistentFlags().BoolVarP(&o.debug, debugFlagName, "d", false, "Debug mode enable")

	_ = cmd.MarkFlagRequired(ocmURLFlagName)
	_ = cmd.MarkFlagRequired(servicesFlagName)
	_ = cmd.MarkFlagRequired(accessTokenFlagName)

	return cmd
}

// Complete initialisation for the server
func (o *serveOptions) Complete(cmd *cobra.Command, args []string) error {

	// ReadFlagsFromFile would read the values of flags from files (if any)
	err := ReadFlagsFromFile(cmd, accessTokenFlagName, servicesFlagName, ocmURLFlagName)
	if err != nil {
		return err
	}

	// Check if debug mode is enabled and set the logging level accordingly
	if o.debug {
		log.SetLevel(log.DebugLevel)
	}

	return nil
}

func (o *serveOptions) Run() error {

	log.Info("Starting ocm-agent server")
	log.WithField("URL", o.ocmURL).Debug("OCM URL configured")
	log.WithField("Service", o.services).Debug("OCM Service configured")

	// create a new router
	r := mux.NewRouter()

	r.HandleFunc("/readyz", HealthCheck).Methods("GET")
	// Add webhook receiver route
	webhookreceiver.AMReceiver().AddRoute(r)

	log.WithField("Port", port).Info("Start listening on port")

	err := http.ListenAndServe(":8081", r)
	if err != nil {
		log.WithError(err).Fatal("OCM Agent failed to serve")
	}

	return nil
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	log.Info("Registering health check end point")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "API is up and running")
}

func initLogging() {
	log.SetLevel(log.Level(logLevel))
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		PadLevelText:  false,
	})
}

func init() {
	// Set default log level
	_ = logLevel.Set(defaultLogLevel)
	cobra.OnInitialize(initLogging)
}
