package sources

import (
	"github.com/gardener/controller-manager-library/pkg/resources"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Source interface {
	resources.Object
	GetTargets(lb resources.Object) (ip, cname string)
	Validate(lb resources.Object) (bool, error)
}

type SourceType interface {
	GetGroupKind() schema.GroupKind
	Get(resources.Object) (Source, error)
}

var SourceTypes = map[schema.GroupKind]SourceType{}
var SourceKinds = []schema.GroupKind{}

func Register(src SourceType) {
	if SourceTypes[src.GetGroupKind()] == nil {
		SourceKinds = append(SourceKinds, src.GetGroupKind())
	}
	SourceTypes[src.GetGroupKind()] = src
}
