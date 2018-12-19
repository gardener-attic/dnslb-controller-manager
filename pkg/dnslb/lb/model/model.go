package model

import (
	"github.com/gardener/lib/pkg/logger"
	"github.com/gardener/lib/pkg/utils"
	"github.com/mandelsoft/dns-controller-manager/pkg/dns/source"
)

type Model struct {
	current *source.DNSCurrentState
	logger.LogContext
	updated utils.StringSet
}

func NewModel(logger logger.LogContext, current *source.DNSCurrentState) *Model {
	return &Model{current, logger, nil}
}

func (this *Model) Check(targets... *Target) bool {
	set:=utils.StringSet{}
	for _, t:=range targets {
		set.Add(t.GetHostName())
	}
	return set.Equals(this.current.Targets)
}

func (this *Model) Apply(targets... *Target) bool {
	set:=utils.StringSet{}
	for _, t:=range targets {
		set.Add(t.GetHostName())
	}
	this.updated=set
	return set.Equals(this.current.Targets)
}

func (this *Model) Get() utils.StringSet {
	return this.updated
}
