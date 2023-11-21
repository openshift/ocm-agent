package ocm

import (
	"fmt"
	sdk "github.com/openshift-online/ocm-sdk-go"
	log "github.com/sirupsen/logrus"
)

// ConnectionBuilder contains the information and logic needed to build a connection to OCM. Don't
// create instances of this type directly; use the NewConnection function instead.
type ConnectionBuilder struct {
	logger           *sdk.Logger
	transportWrapper sdk.TransportWrapper
}

// NewConnection creates a builder that can then be used to configure and build an OCM connection.
// Don't create instances of this type directly; use the NewConnection function instead.
func NewConnection() *ConnectionBuilder {
	return &ConnectionBuilder{}
}

// Build uses the information stored in the builder to create a new OCM connection.
func (b *ConnectionBuilder) Build(baseUrl string, clusterId string, accessToken string) (result *sdk.Connection, err error) {
	builder := sdk.NewConnectionBuilder()

	// Hard-code some values
	builder.URL(baseUrl)

	authToken := fmt.Sprintf("%v:%v", clusterId, accessToken)
	builder.Tokens(authToken)

	if b.logger != nil {
		builder.Logger(*b.logger)
	}
	if b.transportWrapper != nil {
		builder.TransportWrapper(b.transportWrapper)
	}

	// Create the connection:
	result, err = builder.Build()
	if err != nil {
		return result, fmt.Errorf("can't create connection: %v", err)
	}

	return result, nil
}

func (b *ConnectionBuilder) Logger(logger *sdk.Logger) *ConnectionBuilder {
	b.logger = logger
	return b
}

func (b *ConnectionBuilder) TransportWrapper(wrapper sdk.TransportWrapper) *ConnectionBuilder {
	b.transportWrapper = wrapper
	return b
}

// Adapted from https://github.com/gdbranco/rosa/blob/9c5d9a00eef233a7989aca5ddca6762dc0f4d01d/pkg/ocm/clusters.go#L371
func GetInternalIDByExternalID(externalID string, ocm *sdk.Connection) (string, error) {
	log.Debugf("Getting internal ID from external ID %s", externalID)
	query := fmt.Sprintf("external_id = '%s'", externalID)

	response, err := ocm.ClustersMgmt().V1().Clusters().List().
		Search(query).
		Page(1).
		Size(1).
		Send()
	if err != nil {
		log.Error(err)
		return "", err
	}
	if response.Total() < 1 {
		log.Errorf("Cluster with external id %s not found in OCM database.", externalID)
		return "", fmt.Errorf("cluster with external id %s not found in OCM database", externalID)
	}
	cluster := response.Items().Slice()[0]

	return cluster.ID(), nil
}
