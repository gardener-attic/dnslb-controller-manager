// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version

// Version is a global variable which is set during compile time via -ld-flags in the `go build` process.
var Version = "binary was not built properly"
