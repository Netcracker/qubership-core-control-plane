package constancy

import (
	"context"
	"fmt"
	gerrors "github.com/go-errors/errors"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/clustering"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/constancy/fix"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/constancy/migration"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/db"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dr"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model/rest"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"strconv"
	"strings"
)

var ctx context.Context
var log logging.Logger

var flywaySchemaTable = "flyway_schema_history"
var gopgSchemaTable = "gopg_migrations"

// check name of tables of migrations and locks in migrator.NewMigrator
var BunMigrationsTable = "bun_migrations"
var bunLockMigrationTable = "bun_migration_locks"
var bunLockMigrationColumn = "table_name"
var bunInsertLockQuery = `insert into "` + bunLockMigrationTable + `"("` + bunLockMigrationColumn + `") values('` + BunMigrationsTable + `')`

func init() {
	log = logging.GetLogger("constancy")
	ctx = context.Background()
}

//go:generate mockgen -source=storage.go -destination=../test/mock/constancy/stub_storage.go -package=mock_constancy -imports pg=github.com/uptrace/bun
type Storage interface {
	SaveCluster(cluster *domain.Cluster) error
	FindAllClusters() ([]*domain.Cluster, error)
	FindAllRouteConfigs() ([]*domain.RouteConfiguration, error)
	FindAllNodeGroups() ([]*domain.NodeGroup, error)
	FindAllListeners() ([]*domain.Listener, error)
	WithTx(f func(conn *bun.Conn) error) error
	BeforeQuery(ctx context.Context, queryEvent *bun.QueryEvent) context.Context
	AfterQuery(ctx context.Context, queryEvent *bun.QueryEvent)
	FindAllDeploymentVersions() ([]*domain.DeploymentVersion, error)
	FindAllEndpoints() ([]*domain.Endpoint, error)
	FindAllVirtualHosts() ([]*domain.VirtualHost, error)
	FindAllVirtualHostDomains() ([]*domain.VirtualHostDomain, error)
	UpdateVirtualHostDomainByOldDomain(domain *domain.VirtualHostDomain, oldDomain string) error
	FindAllRoutes() ([]*domain.Route, error)
	FindAllHeaderMatchers() ([]*domain.HeaderMatcher, error)
	FindAllHashPolicies() ([]*domain.HashPolicy, error)
	FindAllRetryPolicies() ([]*domain.RetryPolicy, error)
	FindAllClustersNodeGroups() ([]*domain.ClustersNodeGroup, error)
	FindAllListenerWasmFilters() ([]*domain.ListenersWasmFilter, error)
	Generate(uniqEntity domain.Unique) error
	FindAllEnvoyConfigVersions() ([]*domain.EnvoyConfigVersion, error)
	FindAllVirtualHostsDomains() ([]*domain.VirtualHostDomain, error)
	FindAllHealthChecks() ([]*domain.HealthCheck, error)
	FindAllTlsConfigs() ([]*domain.TlsConfig, error)
	FindWasmFilters() ([]*domain.WasmFilter, error)
	FindAllCompositeSatellites() ([]*domain.CompositeSatellite, error)
	FindAllStatefulSessionConfigs() ([]*domain.StatefulSession, error)
	FindAllElectionRecords() ([]*clustering.MasterMetadata, error)
	Migrate(ctx context.Context, db *bun.DB, migrations *migrate.Migrations) error
	FindAllRateLimits() ([]*domain.RateLimit, error)
	FindAllMicroserviceVersions() ([]*domain.MicroserviceVersion, error)
	FindAllExtAuthzFilters() ([]*domain.ExtAuthzFilter, error)
	FindAllCircuitBreakers() ([]*domain.CircuitBreaker, error)
	FindAllThresholds() ([]*domain.Threshold, error)
	FindAllTcpKeepalives() ([]*domain.TcpKeepalive, error)
	FindClusterByName(key string) (*domain.Cluster, error)
	FindTlsConfigByIdAndNodeGroupName(tlsConfigId int32, nodeGroupName string) (*domain.TlsConfigsNodeGroups, error)
	FindClustersNodeGroupByIdAndNodeGroup(clusterId int32, nodeGroup string) (*domain.ClustersNodeGroup, error)
	FindCompositeSatelliteByNamespace(namespace string) (*domain.CompositeSatellite, error)
	FindDeploymentVersionByName(version string) (*domain.DeploymentVersion, error)
	FindEndpointById(id int32) (*domain.Endpoint, error)
	FindHashPolicyById(id int32) (*domain.HashPolicy, error)
	FindHeaderMatcherById(id int32) (*domain.HeaderMatcher, error)
	FindHealthCheckById(id int32) (*domain.HealthCheck, error)
	FindListenerById(id int32) (*domain.Listener, error)
	FindRetryPolicyById(id int32) (*domain.RetryPolicy, error)
	FindRouteConfigById(id int32) (*domain.RouteConfiguration, error)
	FindRouteById(id int32) (*domain.Route, error)
	FindTlsConfigById(id int32) (*domain.TlsConfig, error)
	FindAllTlsConfigsNodeGroups() ([]*domain.TlsConfigsNodeGroups, error)
	FindVirtualHostById(id int32) (*domain.VirtualHost, error)
	FindWasmFilterById(id int32) (*domain.WasmFilter, error)
	FindStatefulSessionById(id int32) (*domain.StatefulSession, error)
	FindRateLimitByNameAndPriority(name string, priority domain.ConfigPriority) (*domain.RateLimit, error)
	FindNodeGroupByName(name string) (*domain.NodeGroup, error)
	FindExtAuthzFilterByName(name string) (*domain.ExtAuthzFilter, error)
	FindCircuitBreakerById(id int32) (*domain.CircuitBreaker, error)
	FindThresholdById(id int32) (*domain.Threshold, error)
	FindTcpKeepaliveById(id int32) (*domain.TcpKeepalive, error)
	FindAllNamespaces() ([]*domain.Namespace, error)
	SaveNamespace(namespace *domain.Namespace) error
	UpdateNamespaceByOldNamespace(namespace *domain.Namespace, oldNamespace string) error
	FindLuaFilters() ([]*domain.LuaFilter, error)
	FindLuaFilterById(id int32) (*domain.LuaFilter, error)
}

type StorageImpl struct {
	DbProvider db.DBProvider
	PGQuery    PGQueryWrapper
	PGConn     PGConnWrapper
	PGTx       PGTxWrapper
}

func setSchemaVersionFromExistingMigrations(schemaVersionTable string, db *bun.DB, migrator *migrate.Migrator, migrations *migrate.Migrations, ctx context.Context) {
	var foundTableName string
	err := db.QueryRow(
		"SELECT to_regclass(?);",
		schemaVersionTable).
		Scan(&foundTableName)

	if len(foundTableName) == 0 {
		return
	}

	sqlRequest := getSqlRequestForTable(schemaVersionTable)
	var lastVersion int64
	err = db.QueryRow(sqlRequest).
		Scan(&lastVersion)
	if err != nil {
		log.Panic("can not get last version from %s: %v", schemaVersionTable, err.Error())
	}

	err = markExistingMigrationsAsApplied(lastVersion, migrator, migrations, ctx)
	if err != nil {
		log.Panic("can not mark existing migrations for %s as applied: %v", schemaVersionTable, err.Error())
	}
}

func getSqlRequestForTable(table string) string {
	if table == flywaySchemaTable {
		return "select version from " + table + " where version <> '' order by version::int desc limit 1;"
	}
	if table == gopgSchemaTable {
		return "select version from " + table + " where version <> 0 order by version::int desc limit 1;"
	}
	return ""
}

func markExistingMigrationsAsApplied(currentSchemaVersion int64, migrator *migrate.Migrator, discoveredMigrations *migrate.Migrations, ctx context.Context) error {
	sorted := discoveredMigrations.Sorted()
	alreadyAppliedMigrations, _ := migrator.AppliedMigrations(ctx)
	for _, currentMigration := range sorted {
		migrationNum, err := strconv.ParseInt(currentMigration.Name, 10, 0)
		if err != nil {
			log.Errorf("Wrong migration name %+v", err.Error())
			return err
		}
		if migrationNum > currentSchemaVersion {
			return nil
		}
		if !contains(alreadyAppliedMigrations, currentMigration) { // in order not to mark migration as applied each time during restart
			err := migrator.MarkApplied(ctx, &currentMigration)
			if err != nil {
				log.Errorf("Error with marking existing migrations as alreadyAppliedMigrations %+v", err.Error())
				return err
			}
		}
	}
	return nil
}

func contains(applied migrate.MigrationSlice, migration migrate.Migration) bool {
	for _, appliedMigration := range applied {
		if migration.Name == appliedMigration.Name {
			return true
		}
	}
	return false
}

// Main constructor
func NewStorage(ctx context.Context, cfg Configurator) *StorageImpl {
	user := cfg.GetDBUserName()
	password := cfg.GetDBPassword()
	database := cfg.GetDBName()
	host := cfg.GetDBHost()
	port := cfg.GetDBPort()
	role := cfg.GetDBRole()

	if user == "" || password == "" || database == "" || host == "" || role == "" {
		log.PanicC(ctx, "Database name, username, password, role or address must not be empty")
	}
	log.InfoC(ctx, "For connection to postgres, using database=%s on host=%s, port=%s with username=%s", database, host, port, user)

	getProvider := func() (db.DBProvider, error) {
		provider := NewDbaasAggregatorLogicalDbProvider(cfg)
		poolOptions := model.PoolOptions{
			LogicalDbProviders: []model.LogicalDbProvider{provider},
		}
		dbPool := dbaasbase.NewDbaaSPool(poolOptions)
		return db.NewDBProvider(dbPool)
	}
	dbProvider, err := getProvider()
	if err != nil {
		log.PanicC(ctx, "Failed to create DBProvider, err = %v", err)
	}
	instance := StorageImpl{dbProvider, &PGQueryWrapperImpl{}, &PGConnWrapperImpl{}, &PGTxWrapperImpl{}}

	db, err := dbProvider.GetDB(ctx)
	if err != nil {
		log.PanicC(ctx, "Failed to acquire DBConn, err = %v", err)
	}

	db.AddQueryHook(instance)

	if dr.GetMode() == dr.Standby {
		log.InfoC(ctx, "Skipping db evolutions because service is running in Standby mode...")
		return &instance
	}

	migrations, err := migration.GetMigrations()
	if err != nil {
		log.ErrorC(ctx, "Failed to discover migrations:%v", err)
		panic(err)
	}

	if err := instance.Migrate(ctx, db, migrations); err != nil {
		if value, ok := err.(*gerrors.Error); ok {
			log.ErrorC(ctx, value.ErrorStack())
		} else {
			log.ErrorC(ctx, "constancy#Migrate failed with unexpected error: %v", err)
		}
		panic(err)
	}

	db.RegisterModel((*domain.ClustersNodeGroup)(nil))
	db.RegisterModel((*domain.ListenersWasmFilter)(nil))
	db.RegisterModel((*domain.TlsConfigsNodeGroups)(nil))
	log.InfoC(ctx, "Registered all domains")

	log.InfoC(ctx, "Start namespaces actualization")
	if err := instance.ActualizeNamespacesInDbAndEnv(ctx); err != nil {
		log.ErrorC(ctx, "Namespaces actualization failed due to error: %v", err)
		panic(err)
	}
	log.InfoC(ctx, "Start namespaces actualization was finished")

	return &instance
}

type DbaasAggregatorLogicalDbProvider struct {
	host     string
	port     string
	username string
	password string
	database string
	tls      string
	role     string
}

func NewDbaasAggregatorLogicalDbProvider(cfg Configurator) *DbaasAggregatorLogicalDbProvider {
	return &DbaasAggregatorLogicalDbProvider{
		host:     cfg.GetDBHost(),
		port:     cfg.GetDBPort(),
		username: cfg.GetDBUserName(),
		password: cfg.GetDBPassword(),
		database: cfg.GetDBName(),
		tls:      cfg.GetDBTls(),
		role:     cfg.GetDBRole(),
	}
}

func (p *DbaasAggregatorLogicalDbProvider) GetOrCreateDb(dbType string, classifier map[string]interface{}, params rest.BaseDbParams) (*model.LogicalDb, error) {
	logicalDB := &model.LogicalDb{}
	connectionProperties := make(map[string]interface{})
	connectionProperties["password"] = p.password
	connectionProperties["username"] = p.username
	connectionProperties["url"] = fmt.Sprintf("postgresql://%s:%v/%s", p.host, p.port, p.database)
	connectionProperties["host"] = p.host
	connectionProperties["tls"] = strings.EqualFold(p.tls, "true")
	connectionProperties["role"] = p.role
	logicalDB.ConnectionProperties = connectionProperties
	logicalDB.Classifier = classifier
	logicalDB.Type = dbType
	return logicalDB, nil
}

func (p *DbaasAggregatorLogicalDbProvider) GetConnection(dbType string, classifier map[string]interface{}, params rest.BaseDbParams) (map[string]interface{}, error) {
	connectionProperties := make(map[string]interface{})
	connectionProperties["password"] = p.password
	connectionProperties["username"] = p.username
	connectionProperties["url"] = fmt.Sprintf("postgresql://%s:%v/%s", p.host, p.port, p.database)
	connectionProperties["host"] = p.host
	connectionProperties["tls"] = strings.EqualFold(p.tls, "true")
	connectionProperties["role"] = p.role

	return connectionProperties, nil
}

func (s *StorageImpl) SaveCluster(cluster *domain.Cluster) error {
	log.DebugC(ctx, "AddCluster cluster to db: %+v", cluster)
	if err := s.WithTx(func(cnn *bun.Conn) error {
		_, err := cnn.NewInsert().Model(&cluster).Exec(ctx)
		return err
	}); err != nil {
		log.ErrorC(ctx, "Error save cluster db: %v", err.Error())
		return err
	}
	return nil
}

func (s *StorageImpl) SaveNamespace(namespace *domain.Namespace) error {
	log.DebugC(ctx, "AddNamespace to db: %+v", namespace)
	if err := s.WithTx(func(cnn *bun.Conn) error {
		_, err := cnn.NewInsert().Model(namespace).Exec(ctx)
		return err
	}); err != nil {
		log.ErrorC(ctx, "Error save namespace db: %v", err.Error())
		return err
	}
	return nil
}

func (s *StorageImpl) UpdateNamespaceByOldNamespace(namespace *domain.Namespace, oldNamespace string) error {
	log.DebugC(ctx, "UpdateNamespace to db: %+v", namespace)
	if err := s.WithTx(func(cnn *bun.Conn) error {
		_, err := cnn.NewUpdate().Model(namespace).Set("namespace = ?", namespace.Namespace).Where("namespace = ?", oldNamespace).Exec(ctx)
		return err
	}); err != nil {
		log.ErrorC(ctx, "Error update namespace db: %v", err.Error())
		return err
	}
	return nil
}

func (s *StorageImpl) UpdateVirtualHostDomainByOldDomain(domain *domain.VirtualHostDomain, oldDomain string) error {
	log.DebugC(ctx, "UpdateVirtualHostDomain to db: %+v", domain)
	if err := s.WithTx(func(cnn *bun.Conn) error {
		_, err := cnn.NewUpdate().Model(domain).Set("domain = ?", domain.Domain).Where("domain = ?", oldDomain).Exec(ctx)
		return err
	}); err != nil {
		log.ErrorC(ctx, "Error update namespace db: %v", err.Error())
		return err
	}
	return nil
}

func (s *StorageImpl) FindAllClusters() ([]*domain.Cluster, error) {
	var result []*domain.Cluster
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Relation("NodeGroups").
			//Relation("Endpoints").
			//Relation("Endpoints.HashPolicies").
			Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Error select all clusters from DB: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllRouteConfigs() ([]*domain.RouteConfiguration, error) {
	var result []*domain.RouteConfiguration
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).
			//Column("route_configuration.*").
			//Relation("VirtualHosts").
			//Relation("VirtualHosts.Routes").
			//Relation("VirtualHosts.Routes.DeploymentVersionVal").
			//Relation("VirtualHosts.Routes.HashPolicies").
			//Relation("VirtualHosts.Routes.HeaderMatchers").
			Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all route counfigrations caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllNodeGroups() ([]*domain.NodeGroup, error) {
	var result []*domain.NodeGroup
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).
			//Relation("Clusters").
			Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all node groups caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllListeners() ([]*domain.Listener, error) {
	var result []*domain.Listener
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all listeners caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) WithTx(f func(conn *bun.Conn) error) error {
	return s.usingDb(func(cnn *bun.Conn) error {
		tx, err := s.PGConn.Begin(cnn)
		if err != nil {
			return err
		}
		defer s.PGTx.Rollback(tx)

		if err := f(cnn); err != nil {
			return err
		}
		return s.PGTx.Commit(tx)
	})
}

func (s *StorageImpl) WithTxBatch(f func(tx BatchStorage) error) error {
	return s.usingDb(func(conn *bun.Conn) error {
		tx, err := s.PGConn.Begin(conn)
		if err != nil {
			return err
		}
		defer s.PGTx.Rollback(tx)

		if err := f(&PgBatchStorage{conn: conn}); err != nil {
			return err
		}
		return s.PGTx.Commit(tx)
	})
}

func (s *StorageImpl) usingDb(f func(conn *bun.Conn) error) error {
	cnn, err := s.DbProvider.GetConn(ctx)
	if err != nil {
		return err
	}
	defer s.PGConn.Close(cnn)
	return f(cnn)
}

func (s *StorageImpl) cleanupSchema(ctx context.Context) error {
	s.WithTx(func(conn *bun.Conn) error {
		_, err := conn.ExecContext(ctx, "delete from clusters")
		return err
	})
	return nil
}

// ===========================
// implement db pg logging interface
// ===========================
func (s StorageImpl) BeforeQuery(ctx context.Context, queryEvent *bun.QueryEvent) context.Context {
	return ctx
}

func (s StorageImpl) AfterQuery(ctx context.Context, queryEvent *bun.QueryEvent) {
	sql := queryEvent.Query
	log.DebugC(ctx, "pg db query: %v", sql)
}

func (s *StorageImpl) FindAllDeploymentVersions() ([]*domain.DeploymentVersion, error) {
	var result []*domain.DeploymentVersion
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Deployment Versions caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllEndpoints() ([]*domain.Endpoint, error) {
	var result []*domain.Endpoint
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Endpoints caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllVirtualHosts() ([]*domain.VirtualHost, error) {
	var result []*domain.VirtualHost
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Virtual Hosts caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllVirtualHostDomains() ([]*domain.VirtualHostDomain, error) {
	var result []*domain.VirtualHostDomain
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Virtual Host Domains caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllNamespaces() ([]*domain.Namespace, error) {
	var result []*domain.Namespace
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Namespaces caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllRoutes() ([]*domain.Route, error) {
	var result []*domain.Route
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Routes caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllHeaderMatchers() ([]*domain.HeaderMatcher, error) {
	var result []*domain.HeaderMatcher
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Header Matchers caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllHashPolicies() ([]*domain.HashPolicy, error) {
	var result []*domain.HashPolicy
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Hash Policies caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllRetryPolicies() ([]*domain.RetryPolicy, error) {
	var result []*domain.RetryPolicy
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Retry Policies caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllClustersNodeGroups() ([]*domain.ClustersNodeGroup, error) {
	var result []*domain.ClustersNodeGroup
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all ClustersNodeGroup caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllListenerWasmFilters() ([]*domain.ListenersWasmFilter, error) {
	var result []*domain.ListenersWasmFilter
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all ListenerWasmFilter caused error: %v", err)
		return nil, err
	}
	return result, nil
}

// TODO performance gap
func (s *StorageImpl) Generate(uniqEntity domain.Unique) error {
	if uniqEntity.GetId() == 0 {
		err := s.WithTx(func(conn *bun.Conn) error {
			var id int32
			err := conn.QueryRowContext(ctx, "select nextval(?)", uniqEntity.TableName()+"_id_seq").Scan(&id)
			if err != nil {
				return err
			}
			uniqEntity.SetId(id)
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *StorageImpl) FindAllEnvoyConfigVersions() ([]*domain.EnvoyConfigVersion, error) {
	var result []*domain.EnvoyConfigVersion
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all EnvoyConfigVersions caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllVirtualHostsDomains() ([]*domain.VirtualHostDomain, error) {
	var result []*domain.VirtualHostDomain
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all VirtualHostDomains caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllHealthChecks() ([]*domain.HealthCheck, error) {
	var result []*domain.HealthCheck
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all HealthChecks caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllTlsConfigs() ([]*domain.TlsConfig, error) {
	var result []*domain.TlsConfig
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all TlsConfigs caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllTlsConfigsNodeGroups() ([]*domain.TlsConfigsNodeGroups, error) {
	var result []*domain.TlsConfigsNodeGroups
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all TlsConfigsNodeGroups caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindWasmFilters() ([]*domain.WasmFilter, error) {
	var result []*domain.WasmFilter
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all WasmFilters caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllCompositeSatellites() ([]*domain.CompositeSatellite, error) {
	var result []*domain.CompositeSatellite
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all CompositeSatellites caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllStatefulSessionConfigs() ([]*domain.StatefulSession, error) {
	var result []*domain.StatefulSession
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all StatefulSessionConfigs caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllElectionRecords() ([]*clustering.MasterMetadata, error) {
	var result []*clustering.MasterMetadata
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all ElectionRecords caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllRateLimits() ([]*domain.RateLimit, error) {
	var result []*domain.RateLimit
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all RateLimits caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllMicroserviceVersions() ([]*domain.MicroserviceVersion, error) {
	var result []*domain.MicroserviceVersion
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all MicroserviceVersions caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllExtAuthzFilters() ([]*domain.ExtAuthzFilter, error) {
	var result []*domain.ExtAuthzFilter
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all ExtAuthzFilters caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllCircuitBreakers() ([]*domain.CircuitBreaker, error) {
	var result []*domain.CircuitBreaker
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all CircuitBreaker caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllThresholds() ([]*domain.Threshold, error) {
	var result []*domain.Threshold
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all Threshold caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) FindAllTcpKeepalives() ([]*domain.TcpKeepalive, error) {
	var result []*domain.TcpKeepalive
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all TcpKeepalives caused error: %v", err)
		return nil, err
	}
	return result, nil
}

func (s *StorageImpl) Migrate(ctx context.Context, db *bun.DB, migrations *migrate.Migrations) error {
	migrator := migrate.NewMigrator(db, migrations, migrate.WithMarkAppliedOnSuccess(true))
	log.Info("Run db evolutions...")
	_ = migrator.Init(ctx) // ignore errors

	log.Info("Lock migrations table")
	lockConn, err := s.DbProvider.GetConn(ctx)
	if err != nil {
		log.ErrorC(ctx, "Error during getting lock connection")
		return gerrors.WrapPrefix(err, "error during getting lock connection", 0)
	}
	defer lockConn.Close()
	lockTx, err := lockConn.BeginTx(ctx, nil)
	if err != nil {
		log.ErrorC(ctx, "Error during beginning lock transaction")
		return gerrors.WrapPrefix(err, "error during beginning lock transaction", 0)
	}
	defer lockTx.Rollback()
	_, err = lockTx.Exec(bunInsertLockQuery)
	if err != nil {
		log.ErrorC(ctx, "Failed to execute lock request")
		return gerrors.WrapPrefix(err, "failed to execute lock request", 0)
	}

	log.Info("apply migrations from flyway...")
	setSchemaVersionFromExistingMigrations(flywaySchemaTable, db, migrator, migrations, ctx)
	log.Info("apply migrations from go-pg...")
	setSchemaVersionFromExistingMigrations(gopgSchemaTable, db, migrator, migrations, ctx)
	log.Info("run migrations...")
	group, err := migrator.Migrate(ctx)
	if err != nil {
		log.ErrorC(ctx, "Db forward migration failed")
		return gerrors.WrapPrefix(err, "db forward migration failed", 0)
	}
	if group.IsZero() {
		log.Info("PersistentStorage is in recent version")
	} else {
		log.Info("PersistentStorage version is evolved for group %s", group)
	}

	if err := fix.Migration17(db, ctx, migrations); err != nil {
		log.Errorf("Failed to apply missed migration #17")
		return gerrors.WrapPrefix(err, "failed to apply missed migration #17", 0)
	}
	if err := fix.Migration24(db); err != nil {
		log.Errorf("Failed to apply missed migration #24")
		return gerrors.WrapPrefix(err, "failed to apply missed migration #24", 0)
	}
	return nil
}

func (s *StorageImpl) ActualizeNamespacesInDbAndEnv(ctx context.Context) error {
	log.InfoC(ctx, "Lock table for namespace actualization")
	lockConn, err := s.DbProvider.GetConn(ctx)
	if err != nil {
		log.ErrorC(ctx, "Error during getting lock connection")
		return gerrors.WrapPrefix(err, "error during getting lock connection", 0)
	}
	defer lockConn.Close()
	lockTx, err := lockConn.BeginTx(ctx, nil)
	if err != nil {
		log.ErrorC(ctx, "Error during beginning lock transaction")
		return gerrors.WrapPrefix(err, "error during beginning lock transaction", 0)
	}
	defer lockTx.Rollback()
	_, err = lockTx.Exec(bunInsertLockQuery)
	if err != nil {
		log.ErrorC(ctx, "Failed to execute lock request")
		return gerrors.WrapPrefix(err, "failed to execute lock request", 0)
	}

	namespacesFromDb, err := s.FindAllNamespaces()
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.ErrorC(ctx, "Error during getting namespaces from db %+v", err.Error())
		return err
	}
	namespaceFromEnv := configloader.GetKoanf().MustString("microservice.namespace")
	if len(namespacesFromDb) > 1 {
		panic("In database more than one namespace")
	}

	if len(namespacesFromDb) > 0 {
		namespaceFromDb := namespacesFromDb[0]
		if namespaceFromDb.Namespace != namespaceFromEnv {
			log.InfoC(ctx, "Namespace in database:%v does not match namespace in env variables:%v. Start replace namespace in virtual host domains", namespaceFromDb, namespaceFromEnv)
			err := s.replaceNamespaceInDomains(namespaceFromEnv)
			if err != nil {
				log.ErrorC(ctx, "Error during replacing namespace in virtual host domains %+v", err.Error())
				return err
			}
			log.InfoC(ctx, "Update namespace in database")
			oldNamespace := namespaceFromDb.Namespace
			namespaceFromDb.Namespace = namespaceFromEnv
			s.UpdateNamespaceByOldNamespace(namespaceFromDb, oldNamespace)
		}
	} else {
		log.InfoC(ctx, "No namespace in database. Namespace in env variables:%v. Start replace namespace in virtual host domains", namespaceFromEnv)
		err := s.replaceNamespaceInDomains(namespaceFromEnv)
		if err != nil {
			log.ErrorC(ctx, "Error during replacing namespace in virtual host domains %+v", err.Error())
			return err
		}
		log.InfoC(ctx, "Save new namespace in database")
		s.SaveNamespace(&domain.Namespace{Namespace: namespaceFromEnv})
	}
	return nil
}

func (s *StorageImpl) replaceNamespaceInDomains(newNamespace string) error {
	virtualHostDomains, err := s.FindAllVirtualHostDomains()
	if err != nil {
		log.ErrorC(ctx, "Error during getting virtual host domains %+v", err.Error())
		return err
	}
	for _, domain := range virtualHostDomains {
		oldDomain := domain.Domain
		var isReplaced bool
		domain.Domain, isReplaced = replaceNamespaceInDomain(domain.Domain, newNamespace)
		if isReplaced {
			s.UpdateVirtualHostDomainByOldDomain(domain, oldDomain)
		}
	}
	return nil
}

func replaceNamespaceInDomain(domain string, namespace string) (newDomain string, isReplaced bool) {
	domainFromDbArray := strings.Split(domain, ":")
	if len(domainFromDbArray) < 2 {
		return domain, false
	}
	domainFromDbArrayWithoutPort := strings.Split(domainFromDbArray[0], ".")
	if len(domainFromDbArrayWithoutPort) < 2 {
		return domain, false
	}
	domainFromDbArrayWithoutPort[1] = namespace
	domainFromDbArray[0] = strings.Join(domainFromDbArrayWithoutPort, ".")
	return strings.Join(domainFromDbArray, ":"), true
}

func (s *StorageImpl) FindLuaFilters() ([]*domain.LuaFilter, error) {
	var result []*domain.LuaFilter
	if err := s.WithTx(func(conn *bun.Conn) error {
		return conn.NewSelect().Model(&result).Scan(ctx)
	}); err != nil {
		log.ErrorC(ctx, "Select all LuaFilters caused error: %v", err)
		return nil, err
	}
	return result, nil
}