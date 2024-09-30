package ocm

import (
	"fmt"
	"net/http"
	"regexp"

	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	slv1 "github.com/openshift-online/ocm-sdk-go/servicelogs/v1"
	"github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/openshift/ocm-agent/pkg/consts"
	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"
)

const (
	OcmOperationIdHeader    = "X-Operation-Id"
	ServiceLogActivePrefix  = "Issue Notification"
	ServiceLogResolvePrefix = "Issue Resolution"
)

type ServiceLogBuilder struct {
	wrappedBuilder *slv1.LogEntryBuilder
	summary        string
	firingDesc     string
	resolveDesc    string
	references     []v1alpha1.NotificationReferenceType
}

type ServiceLog = slv1.LogEntry

func NewServiceLogBuilder(summary, firingDesc, resolveDesc, clusterUUID string, severity v1alpha1.NotificationSeverity, logType string, references []v1alpha1.NotificationReferenceType) *ServiceLogBuilder {
	return &ServiceLogBuilder{
		wrappedBuilder: slv1.NewLogEntry().
			ServiceName(consts.ServiceLogServiceName).
			ClusterUUID(clusterUUID).
			InternalOnly(false).
			Severity(slv1.Severity(severity)).
			LogType(slv1.LogType(logType)),
		summary:     summary,
		firingDesc:  firingDesc,
		resolveDesc: resolveDesc,
		references:  references,
	}
}

var (
	slVarRefRe = regexp.MustCompile(`\${[^{}]*}`)
)

// Replace place holders in the given string with the alert labels and annotations
func replacePlaceHoldersInString(s string, alert *template.Alert) (string, error) {
	var err error
	resolvePlaceHolder := func(placeHolder string) string {
		if err == nil {
			key, value, isOk := placeHolder[2:len(placeHolder)-1], "", false

			if value, isOk = alert.Labels[key]; !isOk {
				if value, isOk = alert.Annotations[key]; !isOk {
					err = fmt.Errorf("alert has no '%s' label or annotation which could be used to replace place holders in the template", key)

					return placeHolder
				}
			}
			return value
		}

		return placeHolder
	}

	return slVarRefRe.ReplaceAllStringFunc(s, resolvePlaceHolder), err
}

func (b *ServiceLogBuilder) Build(firing bool, alert *template.Alert) (*ServiceLog, error) {
	var summary, description string
	var docReferences []string

	// Adjust the summary based on whether the alert is firing or resolved.
	if firing {
		summary = ServiceLogActivePrefix + ": " + b.summary
		description = b.firingDesc
	} else {
		summary = ServiceLogResolvePrefix + ": " + b.summary
		description = b.resolveDesc
	}

	// Replace the place holders in the summary & the description with alert labels & annotations
	if alert != nil {
		var err error

		if summary, err = replacePlaceHoldersInString(summary, alert); err != nil {
			return nil, err
		}
		if description, err = replacePlaceHoldersInString(description, alert); err != nil {
			return nil, err
		}
	}

	// Handle DocReferences
	if len(b.references) > 0 {
		for _, ref := range b.references {
			docReferences = append(docReferences, string(ref))
		}
	}

	// Directly assign the docReferences slice
	logEntry, err := b.wrappedBuilder.
		Summary(summary).
		Description(description).
		DocReferences(docReferences...).
		Build()

	return logEntry, err
}

type OCMClient interface {
	SendServiceLog(logEntry *slv1.LogEntry) error
	SendLimitedSupport(clusterUUID string, lsReason *cmv1.LimitedSupportReason) error
	RemoveLimitedSupport(clusterUUID string, lsReasonID string) error
	GetLimitedSupportReasons(clusterUUID string) ([]*cmv1.LimitedSupportReason, error)
	GetCluster(clusterID string) (*cmv1.Cluster, string, error)
	GetUpgradePolicyState(clusterID string, upgradePolicyID string) (*cmv1.UpgradePolicyState, string, error)
	GetUpgradePolicy(clusterID string, upgradePolicyID string) (*cmv1.UpgradePolicy, string, error)
	GetUpgradePolicies(clusterID string) ([]*cmv1.UpgradePolicy, string, error)
	UpdateUpgradePolicyState(clusterID string, upgradePolicyID string, policyState *cmv1.UpgradePolicyState) (*cmv1.UpgradePolicyState, string, error)
}

type ocmClientImpl struct {
	ocmConnection *sdk.Connection
}

//go:generate mockgen -destination=mocks/ocm.go -package=mocks github.com/openshift/ocm-agent/pkg/ocm OCMClient
func NewOcmClient(ocmConnection *sdk.Connection) OCMClient {
	return &ocmClientImpl{
		ocmConnection: ocmConnection,
	}
}

// https://pkg.go.dev/github.com/openshift-online/ocm-sdk-go@v0.1.382/clustersmgmt/v1#Cluster
func (o *ocmClientImpl) GetCluster(clusterID string) (*cmv1.Cluster, string, error) {
	log.Debugf("Sending get cluster object request to OCM API: %s", clusterID)
	request := o.ocmConnection.ClustersMgmt().V1().Clusters().Cluster(clusterID)
	resp, err := request.Get().Send()
	if err != nil {
		return nil, resp.Header().Get(OcmOperationIdHeader), err
	}

	if resp.Status() < 200 || resp.Status() >= 300 {
		// Extract error details from the resp and return an appropriate error.
		return nil, resp.Header().Get(OcmOperationIdHeader), fmt.Errorf("unexpected status: %d", resp.Status())
	}

	return resp.Body(), resp.Header().Get(OcmOperationIdHeader), nil
}

// GetUpgradePolicy gets a single upgrade policy from a cluster.
// Proxies to https://api.openshift.com/#/default/get_api_clusters_mgmt_v1_clusters__cluster_id__upgrade_policies__upgrade_policy_id_
func (o *ocmClientImpl) GetUpgradePolicy(clusterID string, upgradePolicyID string) (*cmv1.UpgradePolicy, string, error) {
	log.Debugf("Sending get upgrade policy request to OCM API: %s %s", clusterID, upgradePolicyID)
	request := o.ocmConnection.ClustersMgmt().V1().Clusters().Cluster(clusterID).UpgradePolicies().UpgradePolicy(upgradePolicyID)
	resp, err := request.Get().Send()
	if err != nil {
		return nil, resp.Header().Get(OcmOperationIdHeader), err
	}

	if resp.Status() < 200 || resp.Status() >= 300 {
		// Extract error details from the resp and return an appropriate error.
		return nil, resp.Header().Get(OcmOperationIdHeader), fmt.Errorf("unexpected status: %d", resp.Status())
	}
	return resp.Body(), resp.Header().Get(OcmOperationIdHeader), nil
}

// GetUpgradePolicy gets a single upgrade policy's state from a cluster.
// Proxies to https://api.openshift.com#/default/get_api_clusters_mgmt_v1_clusters__cluster_id__upgrade_policies__upgrade_policy_id__state
func (o *ocmClientImpl) GetUpgradePolicyState(clusterID string, upgradePolicyID string) (*cmv1.UpgradePolicyState, string, error) {
	log.Debugf("Sending get upgrade policy state request to OCM API: %s", clusterID)
	request := o.ocmConnection.ClustersMgmt().V1().Clusters().Cluster(clusterID).UpgradePolicies().UpgradePolicy(upgradePolicyID).State()
	resp, err := request.Get().Send()
	if err != nil {
		return nil, resp.Header().Get(OcmOperationIdHeader), err
	}

	if resp.Status() < 200 || resp.Status() >= 300 {
		// Extract error details from the resp and return an appropriate error.
		return nil, resp.Header().Get(OcmOperationIdHeader), fmt.Errorf("unexpected status: %d", resp.Status())
	}
	return resp.Body(), resp.Header().Get(OcmOperationIdHeader), nil
}

// GetUpgradePolicies gets all of the upgrade policies belonging to a cluster from OCM.
// It does not paginate, and sends the whole list as a single list.
// Proxies to https://api.openshift.com/#/default/get_api_clusters_mgmt_v1_clusters__cluster_id__upgrade_policies
func (o *ocmClientImpl) GetUpgradePolicies(clusterID string) ([]*cmv1.UpgradePolicy, string, error) {
	var upgradePolicies []*cmv1.UpgradePolicy
	var operationIdHeader string

	log.Debugf("Sending get all upgrade polices request to OCM API: %s", clusterID)
	collection := o.ocmConnection.ClustersMgmt().V1().Clusters().Cluster(clusterID).UpgradePolicies()
	page := consts.OCMListRequestStartPage
	size := consts.OCMListRequestMaxPerPage

	for {
		resp, err := collection.List().Send()

		if err != nil {
			return nil, resp.Header().Get(OcmOperationIdHeader), err
		}

		if resp.Status() < 200 || resp.Status() >= 300 {
			// Extract error details from the resp and return an appropriate error.
			return nil, resp.Header().Get(OcmOperationIdHeader), fmt.Errorf("unexpected status: %d", resp.Status())
		}

		upgradePolicies = append(upgradePolicies, resp.Items().Slice()...)
		operationIdHeader = resp.Header().Get(OcmOperationIdHeader)
		if resp.Size() < size {
			break
		}
		page++
	}

	return upgradePolicies, operationIdHeader, nil
}

// UpdateUpgradePolicyState updates a single upgrade policy's state for a given cluster.
// Proxies to https://api.openshift.com/#/default/patch_api_clusters_mgmt_v1_clusters__cluster_id__upgrade_policies__upgrade_policy_id__state
func (o *ocmClientImpl) UpdateUpgradePolicyState(clusterID string, upgradePolicyID string, policyState *cmv1.UpgradePolicyState) (*cmv1.UpgradePolicyState, string, error) {
	log.Debugf("Sending update upgrade policy state request to OCM API: %s %s", clusterID, upgradePolicyID)
	request := o.ocmConnection.ClustersMgmt().V1().Clusters().Cluster(clusterID).UpgradePolicies().UpgradePolicy(upgradePolicyID).State().Update().Body(policyState)
	resp, err := request.Send()
	if err != nil {
		return nil, resp.Header().Get(OcmOperationIdHeader), err
	}
	return resp.Body(), resp.Header().Get(OcmOperationIdHeader), nil
}

func (o *ocmClientImpl) SendServiceLog(logEntry *slv1.LogEntry) error {
	// Use the OCM SDK to construct the request for posting a service log for a specific cluster.
	request := o.ocmConnection.ServiceLogs().V1().ClusterLogs().Add().Body(logEntry)

	// Send the request to the OCM API.
	response, err := request.Send()
	if err != nil {
		return fmt.Errorf("can't post service log: %v", err)
	}

	// Check the response status code.
	if response.Status() != http.StatusCreated {
		// Extract error details from the response and return an appropriate error.
		return fmt.Errorf("unexpected status: %d", response.Status())
	}

	return nil
}

func BuildAndSendServiceLog(slBuilder *ServiceLogBuilder, firing bool, alert *template.Alert, ocmClient OCMClient) error {
	logEntry, err := slBuilder.Build(firing, alert)
	if err != nil {
		return err
	}
	return ocmClient.SendServiceLog(logEntry)
}

func (o *ocmClientImpl) SendLimitedSupport(clusterUUID string, lsReason *cmv1.LimitedSupportReason) error {
	internalID, err := GetInternalIDByExternalID(clusterUUID, o.ocmConnection)
	if err != nil {
		return fmt.Errorf("can't get internal id: %w", err)
	}

	response, err := o.ocmConnection.ClustersMgmt().V1().Clusters().Cluster(internalID).LimitedSupportReasons().Add().Body(lsReason).Send()
	if err != nil {
		return fmt.Errorf("can't post limited support: %w", err)
	}

	// Check the response status code
	if response.Status() < 200 || response.Status() >= 300 {
		// Extract error details from the response and return an appropriate error.
		return fmt.Errorf("unexpected status: %d", response.Status())
	}

	return nil
}

func (o *ocmClientImpl) RemoveLimitedSupport(clusterUUID string, lsReasonID string) error {
	internalID, err := GetInternalIDByExternalID(clusterUUID, o.ocmConnection)
	if err != nil {
		return fmt.Errorf("can't get internal id: %w", err)
	}

	response, err := o.ocmConnection.ClustersMgmt().V1().Clusters().Cluster(internalID).LimitedSupportReasons().LimitedSupportReason(lsReasonID).Delete().Send()
	if err != nil {
		return fmt.Errorf("can't delete limited support reason %s from cluster %s: %w", lsReasonID, clusterUUID, err)
	}

	// Check the response status code
	if response.Status() < 200 || response.Status() >= 300 {
		// Extract error details from the response and return an appropriate error.
		return fmt.Errorf("unexpected status: %d", response.Status())
	}

	return nil
}

func (o *ocmClientImpl) GetLimitedSupportReasons(clusterUUID string) ([]*cmv1.LimitedSupportReason, error) {

	internalID, err := GetInternalIDByExternalID(clusterUUID, o.ocmConnection)
	if err != nil {
		return nil, fmt.Errorf("can't get internal id: %w", err)
	}

	response, err := o.ocmConnection.ClustersMgmt().V1().Clusters().Cluster(internalID).LimitedSupportReasons().List().Send()
	if err != nil {
		return nil, fmt.Errorf("can't get limited support reasons: %w", err)
	}

	// Check the response status code
	if response.Status() < 200 || response.Status() >= 300 {
		// Extract error details from the response and return an appropriate error.
		return nil, fmt.Errorf("unexpected status: %d", response.Status())
	}

	return response.Items().Slice(), nil
}
