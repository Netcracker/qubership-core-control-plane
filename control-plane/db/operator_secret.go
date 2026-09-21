package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pgdbaas "github.com/netcracker/qubership-core-lib-go-dbaas-postgres-client/v4"
)

// OperatorSecretsPath is the directory the DBaaS base client scans for Secrets published by the
// DBaaS Operator. It must match mountedSecretPath in qubership-core-lib-go-dbaas-base-client.
const OperatorSecretsPath = "/etc/secrets/dbaas-secrets"

// secretIdentity is the key the base client matches a mounted Secret on, kept in parts so a
// mismatch can be reported field by field.
type secretIdentity struct {
	classifier string // canonical JSON
	dbType     string // lower-cased
	role       string // trimmed
}

func (i secretIdentity) String() string {
	return fmt.Sprintf("classifier=%s type=%s userRole=%q", i.classifier, i.dbType, i.role)
}

// VerifyOperatorSecret returns an error unless a Secret mounted under basePath serves this
// service's database request. It builds the request from buildServiceDbParams, as NewDBProvider and
// password reset do, and applies the base client's matching rules to it. A Secret that passes here
// is therefore the one the client resolves at runtime, instead of falling back to the DBaaS REST API.
func VerifyOperatorSecret(ctx context.Context, basePath string) error {
	params := buildServiceDbParams()
	want, err := newSecretIdentity(params.Classifier(ctx), pgdbaas.DB_TYPE, params.BaseDbParams.Role)
	if err != nil {
		return fmt.Errorf("cannot build the lookup key for this service's database request: %w", err)
	}

	entries, err := os.ReadDir(basePath)
	if err != nil {
		return fmt.Errorf("no DBaaS Operator Secret is mounted: cannot read %s: %w", basePath, err)
	}
	var mounted []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		got, err := readSecretIdentity(filepath.Join(basePath, entry.Name()))
		if err != nil {
			mounted = append(mounted, fmt.Sprintf("%s (%v)", entry.Name(), err))
			continue
		}
		if got == want {
			return nil
		}
		mounted = append(mounted, fmt.Sprintf("%s (%s)", entry.Name(), got))
	}
	if len(mounted) == 0 {
		return fmt.Errorf("no DBaaS Operator Secret is mounted under %s; this service requests %s", basePath, want)
	}
	return fmt.Errorf("no DBaaS Operator Secret under %s matches this service's request %s; mounted: %s",
		basePath, want, strings.Join(mounted, "; "))
}

// readSecretIdentity reads a mounted Secret the way the base client indexes and resolves one: the
// descriptor must be readable, valid, and name a classifier and type, and connectionProperties.json
// must be readable JSON.
func readSecretIdentity(dir string) (secretIdentity, error) {
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return secretIdentity{}, fmt.Errorf("no readable metadata.json")
	}
	var meta struct {
		Classifier map[string]interface{} `json:"classifier"`
		Type       string                 `json:"type"`
		UserRole   string                 `json:"userRole"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return secretIdentity{}, fmt.Errorf("invalid metadata.json: %v", err)
	}
	if len(meta.Classifier) == 0 || meta.Type == "" {
		return secretIdentity{}, fmt.Errorf("metadata.json names no classifier or type")
	}
	props, err := os.ReadFile(filepath.Join(dir, "connectionProperties.json"))
	if err != nil {
		return secretIdentity{}, fmt.Errorf("no readable connectionProperties.json")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(props, &parsed); err != nil {
		return secretIdentity{}, fmt.Errorf("invalid connectionProperties.json: %v", err)
	}
	return newSecretIdentity(meta.Classifier, meta.Type, meta.UserRole)
}

func newSecretIdentity(classifier map[string]interface{}, dbType, role string) (secretIdentity, error) {
	canonical, err := canonicalMap(classifier, true)
	if err != nil {
		return secretIdentity{}, err
	}
	return secretIdentity{classifier: string(canonical), dbType: strings.ToLower(dbType), role: strings.TrimSpace(role)}, nil
}

// canonicalMap mirrors the canonical classifier form of the base client (v3.7.1,
// mounted_secret_provider.go): keys sorted at every level, "scope" lower-cased, empty top-level
// "namespace" and "tenantId" omitted, nil values and empty nested objects dropped, and every other
// value encoded as JSON. The two must agree, or this check accepts a Secret the client rejects.
func canonicalMap(m map[string]interface{}, topLevel bool) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteByte('{')
	first := true
	for _, k := range keys {
		value, err := canonicalValue(k, m[k], topLevel)
		if err != nil {
			return nil, err
		}
		if value == nil {
			continue
		}
		if !first {
			sb.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		sb.Write(key)
		sb.WriteByte(':')
		sb.Write(value)
		first = false
	}
	sb.WriteByte('}')
	return []byte(sb.String()), nil
}

func canonicalValue(key string, v interface{}, topLevel bool) ([]byte, error) {
	switch val := v.(type) {
	case nil:
		return nil, nil
	case string:
		if key == "scope" {
			val = strings.ToLower(val)
		}
		if topLevel && val == "" && (key == "namespace" || key == "tenantId") {
			return nil, nil
		}
		return json.Marshal(val)
	case map[string]interface{}:
		b, err := canonicalMap(val, false)
		if err != nil || string(b) == "{}" {
			return nil, err
		}
		return b, nil
	default:
		return json.Marshal(val)
	}
}
