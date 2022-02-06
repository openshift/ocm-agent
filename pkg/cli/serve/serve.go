package serve

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/ocm-agent/hack"
)

// ServeOptions define the configuration options required by OCM agent to serve.
type ServeOptions struct {
	accessToken string
	services    string
	ocmURL      string
}

var (
	serviceLong = templates.LongDesc(`
	Start the OCM Agent server

	The OCM Agent would receive alerts from AlertManager and post to OCM services such as "Service Log".

	This requires an access token to be able to post to a service in OCM.
	`)

	serviceExample = templates.Examples(`
	# Start the OCM agent server
	ocm-agent server --access-token "$TOKEN" --services "$SERVICE" --ocm-url "https://sample.example.com"

	# Start the OCM agent server by accepting token from a file (value starting with '@' is considered a file)
	ocm-agent server -t @tokenfile --services "$SERVICE" --ocm-url @urlfile
	`)
)

const (
	accessTokenFlagName = "access-token"
	servicesFlagName    = "services"
	ocmURLFlagName      = "ocm-url"
)

func NewServeOptions() *ServeOptions {
	return &ServeOptions{}
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

	_ = cmd.MarkFlagRequired(ocmURLFlagName)
	_ = cmd.MarkFlagRequired(servicesFlagName)
	_ = cmd.MarkFlagRequired(accessTokenFlagName)

	return cmd
}

// Complete initialisation of the flags
func (o *ServeOptions) Complete(cmd *cobra.Command, args []string) error {

	err := ReadFlagsFromFile(cmd, accessTokenFlagName, servicesFlagName, ocmURLFlagName)
	if err != nil {
		return err
	}

	return nil
}

func (o *ServeOptions) Run() error {

	port := 8081

	fmt.Printf("Value of accesstoken is %s\n", o.accessToken)
	fmt.Printf("Value of url is %s\n", o.ocmURL)
	fmt.Printf("Value of service is %s\n", o.services)

	log.Println("starting ocm-agent server")
	// create a new router
	r := mux.NewRouter()

	r.HandleFunc("/readyz", HealthCheck).Methods("GET")
	r.HandleFunc("/data", hack.AddItem).Methods("POST")
	r.HandleFunc("/data", hack.ListItem).Methods("GET")

	log.Printf("Start listening on port %v", port)
	log.Fatal(http.ListenAndServe(":8081", r))

	return nil
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	log.Println("registering health check end point")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "API is up and running")
}
