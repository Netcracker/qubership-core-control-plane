package constancy

import (
	"fmt"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
)

type Configurator interface {
	GetDBName() string
	GetDBUserName() string
	GetDBPassword() string
	GetDBTls() string
	GetDBHost() string
	GetDBPort() string
	GetDBRole() string
}

type PostgresStorageConfigurator struct {
	dbHost     string
	dbPort     string
	dbName     string
	dbUserName string
	dbPassword string
	dbTls      string
	dbRole     string
}

func NewPostgresStorageConfigurator() (*PostgresStorageConfigurator, error) {
	return newPostgresStorageConfigurator(!OperatorSecretMounted())
}

// newPostgresStorageConfigurator reads the legacy pg.* connection properties. They are mandatory
// only when requireCredentials is true. With a DBaaS Operator Secret mounted they are optional,
// because the connection is resolved from that Secret and DB_CREDENTIALS_SECRET may not exist.
func newPostgresStorageConfigurator(requireCredentials bool) (*PostgresStorageConfigurator, error) {
	cfg := &PostgresStorageConfigurator{}
	required := []struct {
		property string
		target   *string
	}{
		{"pg.host", &cfg.dbHost},
		{"pg.port", &cfg.dbPort},
		{"pg.db", &cfg.dbName},
		{"pg.user", &cfg.dbUserName},
		{"pg.passwd", &cfg.dbPassword},
	}
	for _, r := range required {
		*r.target = configloader.GetOrDefaultString(r.property, "")
		if *r.target == "" && requireCredentials {
			return nil, fmt.Errorf("can't find property %s", r.property)
		}
	}
	cfg.dbTls = configloader.GetOrDefaultString("pg.tls", "false")

	cfg.dbRole = configloader.GetOrDefaultString("pg.role", "admin")

	return cfg, nil
}

func (p PostgresStorageConfigurator) GetDBHost() string {
	return p.dbHost
}

func (p PostgresStorageConfigurator) GetDBPort() string {
	return p.dbPort
}

func (p PostgresStorageConfigurator) GetDBName() string {
	return p.dbName
}

func (p PostgresStorageConfigurator) GetDBUserName() string {
	return p.dbUserName
}

func (p PostgresStorageConfigurator) GetDBPassword() string {
	return p.dbPassword
}

func (p PostgresStorageConfigurator) GetDBTls() string {
	return p.dbTls
}

func (p PostgresStorageConfigurator) GetDBRole() string {
	return p.dbRole
}
