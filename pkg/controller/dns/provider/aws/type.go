// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	"fmt"

	"github.com/gardener/dnslb-controller-manager/pkg/config"
	"github.com/gardener/dnslb-controller-manager/pkg/controller/dns/provider"
	. "github.com/gardener/dnslb-controller-manager/pkg/controller/dns/provider"
	"github.com/gardener/dnslb-controller-manager/pkg/log"
	. "github.com/gardener/dnslb-controller-manager/pkg/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

func init() {
	provider.RegisterProviderType("aws", &AWSProviderType{})
}

type AWSProviderType struct {
}

var _ provider.DNSProviderType = &AWSProviderType{}

func (this *AWSProviderType) NewProvider(name string,
	cli_cfg *config.CLIConfig,
	cfg Properties, logctx log.LogCtx) (DNSProvider, error) {

	akid := cfg["AWS_ACCESS_KEY_ID"]
	if akid == "" {
		return nil, fmt.Errorf("'AWS_ACCESS_KEY_ID' required in secret")
	}
	sak := cfg["AWS_SECRET_ACCESS_KEY"]
	if sak == "" {
		return nil, fmt.Errorf("'AWS_SECRET_ACCESS_KEY' required in secret")
	}
	st := cfg["AWS_SESSION_TOKEN"]
	creds := credentials.NewStaticCredentials(akid, sak, st)

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-west-2"),
		Credentials: creds,
	})
	if err != nil {
		return nil, err
	}
	n, err := NewForSession(sess, cli_cfg, logctx)
	if err == nil {
		n.ptype = this
		n.config = cfg
	}
	return n, err
}

func (this *AWSProviderType) NewDefaultProvider(cli_config *config.CLIConfig,
	logctx log.LogCtx) (DNSProvider, error) {
	n, err := New(cli_config, logctx)
	if err == nil {
		n.ptype = this
	}
	return n, err

}
