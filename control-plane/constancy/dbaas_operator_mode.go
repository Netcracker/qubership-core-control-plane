package constancy

import (
	"os"
	"strings"
)

// DbaasOperatorModeEnvVar names the flag the Helm chart sets to "true" when the cluster serves the
// DBaaS Operator CRDs. core-bootstrap reads the same variable with the same meaning.
const DbaasOperatorModeEnvVar = "DBAAS_OPERATOR_ENABLED"

// DbaasOperatorModeEnabled reports whether the DBaaS Operator provisions this service's database. It
// parses the flag the way core-bootstrap does: only "true", in any letter case, enables it.
func DbaasOperatorModeEnabled() bool {
	return strings.ToLower(os.Getenv(DbaasOperatorModeEnvVar)) == "true"
}
