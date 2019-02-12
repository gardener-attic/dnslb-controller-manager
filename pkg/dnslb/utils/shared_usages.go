package utils

import (
	"sync"

	"github.com/gardener/controller-manager-library/pkg/resources"
)

type SharedUsages struct {
	lock   sync.Mutex
	usages map[resources.ClusterObjectKey]resources.ClusterObjectKeySet
}

func NewSharedUsages() *SharedUsages {
	return &SharedUsages{usages: map[resources.ClusterObjectKey]resources.ClusterObjectKeySet{}}
}

func (u *SharedUsages) Get(key resources.ClusterObjectKey) resources.ClusterObjectKeySet {
	u.lock.Lock()
	defer u.lock.Unlock()

	set := u.usages[key]
	if set == nil {
		return resources.ClusterObjectKeySet{}
	}
	return resources.NewClusterObjectKeSetBySets(set)
}

func (u *SharedUsages) Add(key resources.ClusterObjectKey, value resources.ClusterObjectKey) {
	u.lock.Lock()
	defer u.lock.Unlock()

	set := u.usages[key]
	if set == nil {
		set = resources.ClusterObjectKeySet{}
	}
	u.usages[key] = set.Add(value)
}

func (u *SharedUsages) RemoveValue(key resources.ClusterObjectKey) {
	u.lock.Lock()
	defer u.lock.Unlock()

	for _, set := range u.usages {
		delete(set, key)
	}
}
