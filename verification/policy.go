/*
Package verification provides the utilities for handling verification related logic
like Trust Stores and Trust Policies. Few utilities include loading, parsing, and validating
trust policies and trust stores.
*/
package verification

import (
	"errors"
	"fmt"
	"strings"
)

// PolicyDocument represents a trustPolicy.json document
type PolicyDocument struct {
	// Version of the policy document
	Version string `json:"version"`
	// TrustPolicies include each policy statement
	TrustPolicies []TrustPolicy `json:"trustPolicies"`
}

// TrustPolicy represents a policy statement in the policy document
type TrustPolicy struct {
	// Name of the policy statement
	Name string `json:"name"`
	// RegistryScopes that this policy statement affects
	RegistryScopes []string `json:"registryScopes"`
	// SignatureVerification setting for this policy statement
	SignatureVerification string `json:"signatureVerification"`
	// TrustStore this policy statement uses
	TrustStore string `json:"trustStore,omitempty"`
	// TrustedIdentities this policy statement pins
	TrustedIdentities []string `json:"trustedIdentities,omitempty"`
}

func isPresent(val string, values []string) bool {
	for _, v := range values {
		if v == val {
			return true
		}
	}
	return false
}

func validateDistinguishedName(name string) error {

}

func validateTrustedIdentity(identity string, statement TrustPolicy) error {
	if identity == "" {
		return fmt.Errorf("trust policy statement %q has an empty trusted identity", statement.Name)
	}

	if identity != "*" {
		i := strings.Index(identity, ":")
		if i < 0 {
			return fmt.Errorf("trust policy statement %q has trusted identity %q without an identity prefix", statement.Name, statement.TrustStore[:i], statement.TrustStore)
		}

		identityType = identity[:i]

		if identityType == "x509.subject" {
			return validateDistinguishedName(identity[i:])
		}

	}
	// No error
	return nil
}

// ValidatePolicyDocument validates a policy document according to it's version's rule set.
// if any rule is violated, returns an error
func ValidatePolicyDocument(policyDoc *PolicyDocument) error {
	// Constants
	wildcard := "*"
	supportedPolicyVersions := []string{"1.0"}
	supportedVerificationPresets := []string{"strict", "permissive", "audit", "skip"}
	supportedTrustStorePrefixes := []string{"ca"}

	// Validate Version
	if !isPresent(policyDoc.Version, supportedPolicyVersions) {
		return fmt.Errorf("trust policy document uses unsupported version %q", policyDoc.Version)
	}

	// Validate the policy according to 1.0 rules
	if len(policyDoc.TrustPolicies) == 0 {
		return errors.New("trust policy document can not have zero trust policy statements")
	}
	policyStatementNameCount := make(map[string]int)
	registryScopeCount := make(map[string]int)
	for _, statement := range policyDoc.TrustPolicies {

		// Verify statement name is valid
		if statement.Name == "" {
			return errors.New("a trust policy statement is missing a name, every statement requires a name")
		}
		policyStatementNameCount[statement.Name]++

		// Verify registry scopes are valid
		if len(statement.RegistryScopes) == 0 {
			return fmt.Errorf("trust policy statement %q has zero registry scopes, it must specify registry scopes with at least one value", statement.Name)
		}
		if len(statement.RegistryScopes) > 1 && isPresent(wildcard, statement.RegistryScopes) {
			return fmt.Errorf("trust policy statement %q uses wildcard registry scope '*', a wildcard scope cannot be used in conjunction with other scope values", statement.Name)
		}
		for _, scope := range statement.RegistryScopes {
			registryScopeCount[scope]++
		}

		// Verify signature verification preset is valid
		if !isPresent(statement.SignatureVerification, supportedVerificationPresets) {
			return fmt.Errorf("trust policy statement %q uses unsupported signatureVerification value %q", statement.Name, statement.SignatureVerification)
		}

		// Any signature verification other than "skip" needs a trust store
		if statement.SignatureVerification != "skip" && (statement.TrustStore == "" || len(statement.TrustedIdentities) == 0) {
			return fmt.Errorf("trust policy statement %q is either missing a trust store or trusted identities, both must be specified", statement.Name)
		}

		// Verify trust store type is valid if it is present (trust store is optional for "skip" signature verification)
		if statement.TrustStore != "" {
			i := strings.Index(statement.TrustStore, ":")
			if i < 0 || !isPresent(statement.TrustStore[:i], supportedTrustStorePrefixes) {
				return fmt.Errorf("trust policy statement %q uses an unsupported trust store type %q in trust store value %q", statement.Name, statement.TrustStore[:i], statement.TrustStore)
			}
		}

		// If there are trusted identities, verify they are not empty
		for _, identity := range statement.TrustedIdentities {
			if err := validateTrustedIdentity(identity, statement); err != nil {
				return err
			}
		}
		// If there is a wildcard in trusted identies, there shouldn't be any other identities
		if len(statement.TrustedIdentities) > 1 && isPresent(wildcard, statement.TrustedIdentities) {
			return fmt.Errorf("trust policy statement %q uses a wildcard trusted identity '*', a wildcard identity cannot be used in conjunction with other values", statement.Name)
		}
	}

	// Verify unique policy statement names across the policy document
	for key := range policyStatementNameCount {
		if policyStatementNameCount[key] > 1 {
			return fmt.Errorf("multiple trust policy statements use the same name %q, statement names must be unique", key)
		}
	}

	// Verify one policy statement per registry scope
	for key := range registryScopeCount {
		if registryScopeCount[key] > 1 {
			return fmt.Errorf("registry scope %q is present in multiple trust policy statements, one registry scope value can only be associated with one statement", key)
		}
	}

	// No errors
	return nil
}
