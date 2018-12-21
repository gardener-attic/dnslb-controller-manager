package sources

import (
	"github.com/gardener/lib/pkg/resources"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Source interface {
	resources.Object
	GetTargets(lb resources.Object) (ip,cname string)
	Validate(lb resources.Object) (bool, error)
}

type SourceType interface {
	GetGroupKind() schema.GroupKind
	Get(resources.Object) (Source, error)
}

var SourceTypes = map[schema.GroupKind]SourceType{}

func Register(src SourceType) {
	SourceTypes[src.GetGroupKind()]=src
}