package sources

import (
	"github.com/gardener/lib/pkg/utils"
	"github.com/gardener/lib/pkg/resources"
	"github.com/gardener/lib/pkg/logger"
	"k8s.io/apimachinery/pkg/runtime/schema"

	_ "github.com/gardener/dnslb-controller-manager/pkg/dnslb/endpoint/sources/service"

)

type Source interface {
	resources.Object
	GetTargets(logger logger.LogContext, lb resources.Object) (ip,cname string)
	Validate(lb resources.Object) bool
}
}

type SourceType interface {
	GetGroupKind() schema.GroupKind
	Get(resources.Object) Source
}

var SourceTypes map[schema.GroupKind]SourceType{}

func Register(src SourceType) {
	SourceTypes[src.GetGroupKind()]=src
}