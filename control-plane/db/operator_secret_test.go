package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	basemodel "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model/rest"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
)

const testNamespace = "cp-ns"

// operatorMetadata is the descriptor the DBaaS Operator writes for this service's claim: it omits
// userRole when the claim requests the empty role.
const operatorMetadata = `{"classifier":{"microserviceName":"control-plane","namespace":"cp-ns","scope":"service"},"type":"postgresql"}`

const connectionProperties = `{"url":"postgresql://pg:5432/control_plane","username":"u","password":"p"}`

// withServiceNamespace sets microservice.namespace, which the service classifier reads, and restores
// the global configuration when the test ends.
func withServiceNamespace(t *testing.T, namespace string) {
	t.Helper()
	// Registered before t.Setenv, so it runs after the environment is restored.
	t.Cleanup(func() { configloader.Init(configloader.EnvPropertySource()) })
	t.Setenv("MICROSERVICE_NAMESPACE", namespace)
	configloader.Init(configloader.EnvPropertySource())
}

// writeSecret lays out a mounted Secret the way the kubelet projects it. An empty props value
// leaves connectionProperties.json out.
func writeSecret(t *testing.T, basePath, name, metadata, props string) {
	t.Helper()
	dir := filepath.Join(basePath, name)
	assert.NoError(t, os.MkdirAll(dir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(metadata), 0o644))
	if props != "" {
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "connectionProperties.json"), []byte(props), 0o644))
	}
}

func TestVerifyOperatorSecret_AcceptsTheOperatorDescriptor(t *testing.T) {
	withServiceNamespace(t, testNamespace)
	base := t.TempDir()
	writeSecret(t, base, "control-plane-postgresql-service-default-credentials", operatorMetadata, connectionProperties)

	assert.NoError(t, VerifyOperatorSecret(context.Background(), base))
}

func TestVerifyOperatorSecret_IgnoresDescriptiveFields(t *testing.T) {
	withServiceNamespace(t, testNamespace)
	base := t.TempDir()
	writeSecret(t, base, "creds",
		`{"classifier":{"microserviceName":"control-plane","namespace":"cp-ns","scope":"service"},"type":"postgresql",`+
			`"id":"42","name":"dbaas_cp","namespace":"cp-ns","settings":{"pgExtensions":["vector"]}}`,
		connectionProperties)

	assert.NoError(t, VerifyOperatorSecret(context.Background(), base))
}

func TestVerifyOperatorSecret_AppliesTheClientCanonicalForm(t *testing.T) {
	withServiceNamespace(t, testNamespace)
	base := t.TempDir()
	// Key order, the case of "scope", and the case of the type are not part of the identity.
	writeSecret(t, base, "creds",
		`{"type":"PostgreSQL","classifier":{"scope":"SERVICE","namespace":"cp-ns","microserviceName":"control-plane"}}`,
		connectionProperties)

	assert.NoError(t, VerifyOperatorSecret(context.Background(), base))
}

func TestVerifyOperatorSecret_FindsTheMatchAmongOtherSecrets(t *testing.T) {
	withServiceNamespace(t, testNamespace)
	base := t.TempDir()
	writeSecret(t, base, "a-other-service",
		`{"classifier":{"microserviceName":"site-management","namespace":"cp-ns","scope":"service"},"type":"postgresql"}`,
		connectionProperties)
	writeSecret(t, base, "b-control-plane", operatorMetadata, connectionProperties)

	assert.NoError(t, VerifyOperatorSecret(context.Background(), base))
}

func TestVerifyOperatorSecret_RejectsMismatches(t *testing.T) {
	for _, tc := range []struct {
		name     string
		metadata string
		want     string
	}{
		{
			name:     "explicit role",
			metadata: `{"classifier":{"microserviceName":"control-plane","namespace":"cp-ns","scope":"service"},"type":"postgresql","userRole":"admin"}`,
			want:     `userRole="admin"`,
		},
		{
			name:     "extra classifier key",
			metadata: `{"classifier":{"microserviceName":"control-plane","namespace":"cp-ns","scope":"service","dbClassifier":"default"},"type":"postgresql"}`,
			want:     `"dbClassifier":"default"`,
		},
		{
			name:     "other namespace",
			metadata: `{"classifier":{"microserviceName":"control-plane","namespace":"elsewhere","scope":"service"},"type":"postgresql"}`,
			want:     `"namespace":"elsewhere"`,
		},
		{
			name:     "other owner",
			metadata: `{"classifier":{"microserviceName":"config-server","namespace":"cp-ns","scope":"service"},"type":"postgresql"}`,
			want:     `"microserviceName":"config-server"`,
		},
		{
			name:     "other type",
			metadata: `{"classifier":{"microserviceName":"control-plane","namespace":"cp-ns","scope":"service"},"type":"mongodb"}`,
			want:     `type=mongodb`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withServiceNamespace(t, testNamespace)
			base := t.TempDir()
			writeSecret(t, base, "creds", tc.metadata, connectionProperties)

			err := VerifyOperatorSecret(context.Background(), base)

			assert.Error(t, err)
			// The message names both what was requested and what was mounted.
			assert.Contains(t, err.Error(), `classifier={"microserviceName":"control-plane","namespace":"cp-ns","scope":"service"} type=postgresql userRole=""`)
			assert.Contains(t, err.Error(), "creds (")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestVerifyOperatorSecret_RejectsUnusableSecrets(t *testing.T) {
	for _, tc := range []struct {
		name     string
		metadata string
		props    string
		want     string
	}{
		{"missing connection properties", operatorMetadata, "", "no readable connectionProperties.json"},
		{"invalid connection properties", operatorMetadata, "{", "invalid connectionProperties.json"},
		{"invalid metadata", "{", connectionProperties, "invalid metadata.json"},
		{"incomplete metadata", `{"type":"postgresql"}`, connectionProperties, "names no classifier or type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withServiceNamespace(t, testNamespace)
			base := t.TempDir()
			writeSecret(t, base, "creds", tc.metadata, tc.props)

			err := VerifyOperatorSecret(context.Background(), base)

			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestVerifyOperatorSecret_RejectsAMissingMount(t *testing.T) {
	withServiceNamespace(t, testNamespace)

	err := VerifyOperatorSecret(context.Background(), filepath.Join(t.TempDir(), "absent"))

	assert.ErrorContains(t, err, "no DBaaS Operator Secret is mounted")
}

func TestVerifyOperatorSecret_RejectsAnEmptyMount(t *testing.T) {
	withServiceNamespace(t, testNamespace)
	base := t.TempDir()
	// A stray file at the top level is not a Secret directory.
	assert.NoError(t, os.WriteFile(filepath.Join(base, "metadata.json"), []byte(operatorMetadata), 0o644))

	err := VerifyOperatorSecret(context.Background(), base)

	assert.ErrorContains(t, err, "no DBaaS Operator Secret is mounted under")
}

// recordingDbaasClient captures the identity a caller requests.
type recordingDbaasClient struct {
	classifier map[string]interface{}
	params     rest.BaseDbParams
}

func (c *recordingDbaasClient) GetOrCreateDb(context.Context, string, map[string]interface{}, rest.BaseDbParams) (*basemodel.LogicalDb, error) {
	return nil, errors.New("not expected in this test")
}

func (c *recordingDbaasClient) GetConnection(_ context.Context, _ string, classifier map[string]interface{}, params rest.BaseDbParams) (map[string]interface{}, error) {
	c.classifier, c.params = classifier, params
	return map[string]interface{}{"password": "rotated"}, nil
}

// Password reset must request the same identity as the main connection. If the two diverged, a
// credential rotation would miss the mounted Secret that VerifyOperatorSecret checked at startup.
func TestGetPasswordFromDbaas_RequestsTheServiceIdentity(t *testing.T) {
	withServiceNamespace(t, testNamespace)
	client := &recordingDbaasClient{}
	resetter := &pgDbWithPasswordReset{dbaasClient: client}

	password, err := resetter.getPasswordFromDbaas(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "rotated", password)
	service := buildServiceDbParams()
	assert.Equal(t, service.Classifier(context.Background()), client.classifier)
	assert.Equal(t, service.BaseDbParams, client.params)
}
