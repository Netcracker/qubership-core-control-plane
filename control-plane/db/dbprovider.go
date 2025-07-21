package db

import (
	"context"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"

	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	pgdbaas "github.com/netcracker/qubership-core-lib-go-dbaas-postgres-client/v4"
	"github.com/netcracker/qubership-core-lib-go-dbaas-postgres-client/v4/model"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/uptrace/bun"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("db")
}

//go:generate mockgen -source=dbprovider.go -destination=../test/mock/db/stub_dbprovider.go -package=mock_db -imports pg=github.com/uptrace/bun
type DBProvider interface {
	GetDB(ctx context.Context) (*bun.DB, error)
	GetConn(ctx context.Context) (*bun.Conn, error)
	NewDB(subscribers ...func(db PgDB, event ConnEvent, err error)) (PgDB, error)
	Listen(channel string, connectCallback func(), notificationCallback func(payload string)) (PersistentStorageListener, error)
}

type ConnEvent int

const (
	Initialized ConnEvent = iota
	PasswordReset
	Error
)

type DefaultDBProvider struct {
	pgClient  pgdbaas.PgClient
	serviceDB pgdbaas.Database
	dbPool    dbaasbase.DbaaSClient
}

func NewDBProvider(dbPool *dbaasbase.DbaaSPool) (*DefaultDBProvider, error) {
	pgDbaasClient := pgdbaas.NewClient(dbPool)
	params := buildServiceDbParams()
	database := pgDbaasClient.ServiceDatabase(params)
	pgClient, err := database.GetPgClient()
	if err != nil {
		return nil, err
	}
	return &DefaultDBProvider{pgClient: pgClient, serviceDB: database, dbPool: dbPool}, nil
}

func (p *DefaultDBProvider) GetDB(ctx context.Context) (*bun.DB, error) {
	return p.pgClient.GetBunDb(ctx)
}

func (p *DefaultDBProvider) GetConn(ctx context.Context) (*bun.Conn, error) {
	db, err := p.pgClient.GetBunDb(ctx)
	if err != nil {
		return nil, err
	}
	db.RegisterModel((*domain.ClustersNodeGroup)(nil))
	db.RegisterModel((*domain.ListenersWasmFilter)(nil))
	db.RegisterModel((*domain.TlsConfigsNodeGroups)(nil))
	conn, err := db.Conn(ctx)
	return &conn, err
}

func buildServiceDbParams() model.DbParams {
	return model.DbParams{
		Classifier: createControlPlaneServiceClassifier,
	}
}

func createControlPlaneServiceClassifier(ctx context.Context) map[string]interface{} {
	namespace := configloader.GetKoanf().MustString("microservice.namespace")
	return map[string]interface{}{
		"namespace":        namespace,
		"microserviceName": "control-plane",
		"scope":            "service",
	}
}
