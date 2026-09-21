package constancy

import (
	"os"
	"strings"
)

// OperatorModeEnvVar names the flag the Helm chart sets to "true" when the cluster serves the
// DBaaS Operator CRDs. core-bootstrap reads the same variable with the same meaning.
const OperatorModeEnvVar = "DBAAS_OPERATOR_ENABLED"

// OperatorModeEnabled reports whether the DBaaS Operator provisions this service's database. It
// parses the flag the way core-bootstrap does: only "true", in any letter case, enables it.
func OperatorModeEnabled() bool {
	return strings.ToLower(os.Getenv(OperatorModeEnvVar)) == "true"
}
