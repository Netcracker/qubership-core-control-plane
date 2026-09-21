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
	cfg := &PostgresStorageConfigurator{}
	cfg.dbHost = configloader.GetOrDefaultString("pg.host", "")
	if cfg.dbHost == "" {
		return nil, fmt.Errorf("can't find property pg.host")
	}

	cfg.dbPort = configloader.GetOrDefaultString("pg.port", "")
	if cfg.dbPort == "" {
		return nil, fmt.Errorf("can't find property pg.port")
	}

	cfg.dbName = configloader.GetOrDefaultString("pg.db", "")
	if cfg.dbName == "" {
		return nil, fmt.Errorf("can't find property pg.db")
	}

	cfg.dbUserName = configloader.GetOrDefaultString("pg.user", "")
	if cfg.dbUserName == "" {
		return nil, fmt.Errorf("can't find property pg.user")
	}

	cfg.dbPassword = configloader.GetOrDefaultString("pg.passwd", "")
	if cfg.dbPassword == "" {
		return nil, fmt.Errorf("can't find property pg.passwd")
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
