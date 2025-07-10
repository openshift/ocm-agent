FROM registry.ci.openshift.org/openshift/release:rhel-8-release-golang-1.22-openshift-4.18 AS builder

RUN mkdir -p /workdir
WORKDIR /workdir
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM registry.access.redhat.com/ubi8-minimal:8.10-1255

LABEL io.openshift.managed.name="ocm-agent" \
  io.openshift.managed.description="Agent to interact with OCM on managed clusters"

COPY --from=builder /workdir/build/_output/ocm-agent /usr/local/bin/

ADD build/bin/* /usr/local/bin/

ENV USER_UID=1000 \
  USER_NAME=ocm-agent
RUN /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT [ "/usr/local/bin/entrypoint" ]
