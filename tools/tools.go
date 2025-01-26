//go:build tools
// +build tools

// Copyright 2024 Authors of elf-io
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
