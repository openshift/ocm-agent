# FAQ

**What does OA stand for?**

ocm-agent, also OAO stands for ocm-agent-operator.

**What is ocm-agent?**

ocm-agent is a web-service managed by OAO as a funnel point for all managed clusters related to any OCM interaction.

**What are ocm-agent use-cases?**

1. Provide a service endpoint for the publishing of service logs to the customer from other on-cluster workloads.

> **_NOTE_** OCM Agent use-cases will grow as the project adds abilities.

**Does OA silence alerts?**

Yes, but instead of sending the alerts to PagerDuty and page the primary on-call, the OCM Agent will redirect the selected alerts to an embedded alertmanager-receiver, and have the service-log sent automatically.

**How does OA determine which service log template to use?**

OAO manages `managed-notifications` API and in turn managednotifications CRD, in association with the CR when an alert is on-boarded will have a config to store template and will be watched by ocm-agent to send the service log.
