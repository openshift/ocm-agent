FROM registry.ci.openshift.org/openshift/release:rhel-9-release-golang-1.23-openshift-4.19 AS builder

RUN mkdir -p /workdir
WORKDIR /workdir
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.6-1758184547

LABEL io.openshift.managed.name="ocm-agent" \
  io.openshift.managed.description="Agent to interact with OCM on managed clusters"

COPY --from=builder /workdir/build/_output/ocm-agent /usr/local/bin/

ADD build/bin/* /usr/local/bin/

ENV USER_UID=1000 \
  USER_NAME=ocm-agent
RUN /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT [ "/usr/local/bin/entrypoint" ]
