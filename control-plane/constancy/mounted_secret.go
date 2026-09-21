package constancy

import (
	"os"
	"path/filepath"
)

// dbaasMountedSecretsPath is the directory that the DBaaS base client scans for Secrets
// materialized by the DBaaS Operator from a DatabaseSecretClaim. It must stay in sync with
// mountedSecretPath in qubership-core-lib-go-dbaas-base-client.
const dbaasMountedSecretsPath = "/etc/secrets/dbaas-secrets"

// dbaasSecretMetadataFile is the descriptor the DBaaS Operator writes next to
// connectionProperties.json. The base client matches a Secret to a request by its contents, so a
// directory without this file is not a usable database Secret.
const dbaasSecretMetadataFile = "metadata.json"

// hasMountedDbaasSecret reports whether at least one DBaaS Operator Secret is mounted under
// basePath.
//
// DbaaSPool appends its own mounted-secret provider after the providers the application supplies,
// and the first provider that returns a result wins. DbaasAggregatorLogicalDbProvider always
// returns the credentials it reads from /etc/secrets/pod-secrets, so without this check the
// mounted-secret provider is unreachable. When an operator-managed Secret is mounted, the local
// provider steps aside and lets the base client resolve the database from it, keeping the REST
// call as the last fallback.
func hasMountedDbaasSecret(basePath string) bool {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := os.Stat(filepath.Join(basePath, entry.Name(), dbaasSecretMetadataFile))
		if err == nil && info.Size() > 0 {
			return true
		}
	}
	return false
}
