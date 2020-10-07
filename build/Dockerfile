# SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

FROM alpine:3.7

RUN apk add --update bash curl
RUN mkdir /plugins

ADD bin/rel/dnslb-controller-manager /dnslb-controller-manager

WORKDIR /

ENTRYPOINT ["/dnslb-controller-manager"]
