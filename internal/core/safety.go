// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrReadOnly is returned when a write operation is attempted while the
// cluster or global configuration is in read-only mode.
var ErrReadOnly = errors.New("operation not permitted: read-only mode enabled")

// reValidName matches Kubernetes DNS-subdomain names as per RFC 1123:
// lowercase alphanumeric characters, '-', or '.', start/end with alphanumeric.
// Maximum length is 253 characters.
var reValidName = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?$`)

// reValidNamespace matches Kubernetes namespace names: DNS label format.
// lowercase alphanumeric and '-', max 63 characters.
var reValidNamespace = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`)

// ValidateResourceName validates a Kubernetes resource name.
// Rules:
//   - Must not be empty.
//   - Maximum 253 characters (DNS subdomain limit).
//   - Only lowercase alphanumeric, '-', and '.'.
//   - Must begin and end with an alphanumeric character.
func ValidateResourceName(name string) error {
	if name == "" {
		return errors.New("resource name must not be empty")
	}
	if len(name) > 253 {
		return fmt.Errorf("resource name %q exceeds maximum length of 253 characters", name)
	}
	if !reValidName.MatchString(name) {
		return fmt.Errorf("resource name %q is invalid: must consist of lowercase alphanumeric characters, '-', or '.', and must start and end with an alphanumeric character", name)
	}
	return nil
}

// ValidateNamespace validates a Kubernetes namespace name.
// Rules:
//   - Must not be empty.
//   - Maximum 63 characters (DNS label limit).
//   - Only lowercase alphanumeric and '-'.
//   - Must begin and end with an alphanumeric character.
func ValidateNamespace(namespace string) error {
	if namespace == "" {
		return errors.New("namespace must not be empty")
	}
	if len(namespace) > 63 {
		return fmt.Errorf("namespace %q exceeds maximum length of 63 characters", namespace)
	}
	if !reValidNamespace.MatchString(namespace) {
		return fmt.Errorf("namespace %q is invalid: must consist of lowercase alphanumeric characters or '-', and must start and end with an alphanumeric character", namespace)
	}
	return nil
}

// guardReadOnly returns ErrReadOnly when readOnly is true.
// It is used by write operations to enforce the read-only flag before any
// cluster API call is made.
func guardReadOnly(readOnly bool) error {
	if readOnly {
		return ErrReadOnly
	}
	return nil
}

// isSystemNamespace reports whether ns is one of the well-known Kubernetes
// system namespaces that should be protected from accidental modification.
func isSystemNamespace(ns string) bool {
	switch strings.ToLower(ns) {
	case "kube-system", "kube-public", "kube-node-lease":
		return true
	}
	return false
}
