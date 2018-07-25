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

package controller

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"plugin"

	"github.com/sirupsen/logrus"

	. "github.com/gardener/dnslb-controller-manager/pkg/utils"
)

var Plugins = map[string]*plugin.Plugin{}

func LoadPlugins(dir string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read plugin dir '%s': %s", dir, err)
	}
	logrus.Infof("scanning %s for plugins", dir)
	for _, f := range files {
		path := fmt.Sprintf("%s%c%s", dir, filepath.Separator, f.Name())
		if ok, _ := IsFile(path); ok {
			p, err := plugin.Open(path)
			if err != nil {
				logrus.Errorf("cannot load plugin %s: %s", path, err)
			} else {
				v, err := p.Lookup("Name")
				if err != nil {
					logrus.Errorf("loaded plugin %s has no name", path)
				} else {
					name, ok := v.(*string)
					if ok {
						logrus.Infof("loaded plugin %s from %s", *name, path)
						Plugins[*name] = p
					} else {
						logrus.Errorf("loaded plugin %s has invalid variable Name: %s", path)
					}
				}
			}
		}
	}
	return nil
}
