package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	gerrors "github.com/go-errors/errors"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/constancy"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/db"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	asrt "github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

const PostgresDataBaseUser = "postgres"
const PostgresDataBasePassword = "12345"

var (
	dbPort string

	logTest = logging.GetLogger("test_migration_cp")
)

//
//func Test_Migration(t *testing.T) {
//	assert := asrt.New(t)
//	testMigration(assert, "test_migration", false)
//}
//
//func Test_Migration_With_Fail(t *testing.T) {
//	assert := asrt.New(t)
//	testMigration(assert, "test_migration_with_fail", true)
//}
//
//func Test_ValidateNamespaceWithNamespace(t *testing.T) {
//	assert := asrt.New(t)
//	testValidateNamespace(assert, "test_validate_ns", true)
//}
//
//func Test_ValidateNamespaceWithoutNamespace(t *testing.T) {
//	assert := asrt.New(t)
//	testValidateNamespace(assert, "test_validate_ns_without_ns", false)
//}

func createPostgresDatabase(databaseName string) error {
	pgResource, found := cm.containers[Postgres]
	if !found {
		logTest.ErrorC(ctx, "Could not find pg container for storage test")
		return errors.New("could not find pg container for storage test")
	}
	return runPgQuery(pgResource, fmt.Sprintf("CREATE DATABASE %s;", databaseName))
}

type TestPostgresStorageConfigurator struct {
	dbHost     string
	dbPort     string
	dbName     string
	dbUserName string
	dbPassword string
	dbTls      string
	dbRole     string
}

func NewTestPostgresStorageConfigurator(dataBaseName string) (*TestPostgresStorageConfigurator, error) {
	cfg := &TestPostgresStorageConfigurator{}
	cfg.dbHost = dockerHost
	cfg.dbPort = dbPort
	cfg.dbName = dataBaseName
	cfg.dbUserName = PostgresDataBaseUser
	cfg.dbPassword = PostgresDataBasePassword
	cfg.dbTls = "false"
	cfg.dbRole = "admin"
	return cfg, nil
}

func (p TestPostgresStorageConfigurator) GetDBHost() string {
	return p.dbHost
}

func (p TestPostgresStorageConfigurator) GetDBPort() string {
	return p.dbPort
}

func (p TestPostgresStorageConfigurator) GetDBName() string {
	return p.dbName
}

func (p TestPostgresStorageConfigurator) GetDBUserName() string {
	return p.dbUserName
}

func (p TestPostgresStorageConfigurator) GetDBPassword() string {
	return p.dbPassword
}

func (p TestPostgresStorageConfigurator) GetDBTls() string {
	return p.dbTls
}

func (p TestPostgresStorageConfigurator) GetDBRole() string {
	return p.dbRole
}

type TestEntity0 struct {
	bun.BaseModel `bun:"test_entity"`
	Id            int32
	Migration0    string `bun:"migration0"`
}

type TestEntity1 struct {
	bun.BaseModel `bun:"test_entity"`
	Id            int32
	Migration0    string `bun:"migration0"`
	Migration1    string `bun:"migration1"`
}

type TestEntity2 struct {
	bun.BaseModel `bun:"test_entity"`
	Id            int32
	Migration0    string `bun:"migration0"`
	Migration1    string `bun:"migration1"`
	Migration2    string `bun:"migration2"`
}

type Lock struct {
	bun.BaseModel `bun:"table:bun_migration_locks"`

	TableName string `bun:"table_name,pk"`
}

func testMigration(assert *asrt.Assertions, databaseName string, fail bool) {
	assert.Nil(createPostgresDatabase(databaseName))

	instance := createTestStorage(databaseName)

	db, err := instance.DbProvider.GetDB(ctx)
	if err != nil {
		logTest.Panic("Failed to acquire DBConn, err = %v", err)
	}

	db.AddQueryHook(instance)

	if fail {
		migrationsWithFail := createTestMigrations(true)
		if err := instance.Migrate(ctx, db, migrationsWithFail); err != nil {
			if value, ok := err.(*gerrors.Error); ok {
				logTest.Error(value.ErrorStack())
			} else {
				logTest.Errorf("constancy#Migrate failed with unexpected error: %v", err)
			}
			if err.Error() != "db forward migration failed: fail migration manually" {
				panic(err)
			}
		}
	}

	migrationsWithoutFail := createTestMigrations(false)

	if err := instance.Migrate(ctx, db, migrationsWithoutFail); err != nil {
		if value, ok := err.(*gerrors.Error); ok {
			logTest.Error(value.ErrorStack())
		} else {
			logTest.Errorf("constancy#Migrate failed with unexpected error: %v", err)
		}
		panic(err)
	}

	resultSlice := make([]TestEntity2, 0)

	err = db.NewSelect().
		ColumnExpr("*").
		Model(&resultSlice).
		ModelTableExpr("test_entity").
		Scan(ctx)
	if err != nil {
		panic(err)
	}

	logTest.Infof("SELECT from "+databaseName+":%v", resultSlice)

	assert.Equal(1, len(resultSlice))
	assert.Equal("migration0", resultSlice[0].Migration0)
	assert.Equal("migration1", resultSlice[0].Migration1)
	assert.Equal("migration2", resultSlice[0].Migration2)

	if fail {
		var ms migrate.MigrationSlice

		err = db.NewSelect().
			ColumnExpr("*").
			Model(&ms).
			ModelTableExpr(constancy.BunMigrationsTable).
			Scan(ctx)
		if err != nil {
			panic(err)
		}

		assert.Equal(int64(1), ms[0].GroupID)
		assert.Equal(int64(2), ms[1].GroupID)
		assert.Equal(int64(2), ms[2].GroupID)
	}
}

func testValidateNamespace(assert *asrt.Assertions, dataBaseName string, withNamespace bool) {
	assert.Nil(createPostgresDatabase(dataBaseName))

	instance := createTestStorage(dataBaseName)

	db, err := instance.DbProvider.GetDB(ctx)
	if err != nil {
		logTest.Panic("Failed to acquire DBConn, err = %v", err)
	}

	domain1 := &domain.VirtualHostDomain{
		Domain:        "*",
		Version:       1,
		VirtualHostId: 2,
	}

	domain2 := &domain.VirtualHostDomain{
		Domain:        "test-service:8080",
		Version:       1,
		VirtualHostId: 2,
	}

	domain3 := &domain.VirtualHostDomain{
		Domain:        "test-service.test-namespace-before:8080",
		Version:       1,
		VirtualHostId: 2,
	}

	domain4 := &domain.VirtualHostDomain{
		Domain:        "test-service.test-namespace-before.bla:8080",
		Version:       1,
		VirtualHostId: 2,
	}

	namespace := &domain.Namespace{}

	_, err = db.NewCreateTable().Model(&Lock{}).Exec(ctx)
	if err != nil {
		panic(err)
	}

	db.AddQueryHook(instance)

	_, err = db.NewCreateTable().Model(namespace).Exec(ctx)
	if err != nil {
		panic(err)
	}

	if withNamespace {
		namespace.Namespace = "test-namespace-before"
		_, err = db.NewInsert().Model(namespace).Exec(ctx)
		if err != nil {
			panic(err)
		}
	}

	_, err = db.NewCreateTable().Model(domain1).Exec(ctx)
	if err != nil {
		panic(err)
	}

	_, err = db.NewInsert().Model(domain1).Exec(ctx)
	if err != nil {
		panic(err)
	}
	_, err = db.NewInsert().Model(domain2).Exec(ctx)
	if err != nil {
		panic(err)
	}
	_, err = db.NewInsert().Model(domain3).Exec(ctx)
	if err != nil {
		panic(err)
	}
	_, err = db.NewInsert().Model(domain4).Exec(ctx)
	if err != nil {
		panic(err)
	}

	instance.ActualizeNamespacesInDbAndEnv(context.Background())

	var domains []*domain.VirtualHostDomain
	err = db.NewSelect().Model(&domains).Scan(ctx)
	if err != nil {
		panic(err)
	}

	var d1, d2, d3, d4 bool

	for _, hostDomain := range domains {
		if hostDomain.Domain == "*" {
			d1 = true
		}
		if hostDomain.Domain == "test-service:8080" {
			d2 = true
		}
		if hostDomain.Domain == "test-service.test-control-plane:8080" {
			d3 = true
		}
		if hostDomain.Domain == "test-service.test-control-plane.bla:8080" {
			d4 = true
		}
	}

	assert.True(d1)
	assert.True(d2)
	assert.True(d3)
	assert.True(d4)
}

func createTestStorage(dataBaseName string) constancy.StorageImpl {
	pgContainer := cm.containers[Postgres]
	dbPort = fmt.Sprintf("%v", pgContainer.Ports[5432])
	//It is needed to avoid NPE during creation dbaas client during creation dbaas pool
	//namespace := configloader.GetKoanf().MustString("microservice.namespace")
	//setEnvIfNotSet("microservice.namespace", "test-control-plane")
	//configloader.Init(configloader.EnvPropertySource())

	cfg, err := NewTestPostgresStorageConfigurator(dataBaseName)
	if err != nil {
		panic(err)
	}

	getProvider := func() (db.DBProvider, error) {
		provider := constancy.NewDbaasAggregatorLogicalDbProvider(cfg)
		poolOptions := model.PoolOptions{
			LogicalDbProviders: []model.LogicalDbProvider{provider},
		}
		dbPool := dbaasbase.NewDbaaSPool(poolOptions)
		return db.NewDBProvider(dbPool)
	}
	dbProvider, err := getProvider()
	if err != nil {
		logTest.Panic("Failed to create DBProvider, err = %v", err)
	}
	return constancy.StorageImpl{dbProvider, &constancy.PGQueryWrapperImpl{}, &constancy.PGConnWrapperImpl{}, &constancy.PGTxWrapperImpl{}}
}

func createTestMigrations(fail bool) *migrate.Migrations {
	migrations := &migrate.Migrations{}
	migration0 := migrate.Migration{Name: "00000000000000", Comment: "first_migration", Up: func(ctx context.Context, db *bun.DB) error {
		logTest.Info("first_migration")
		err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			testEntity0 := &TestEntity0{
				BaseModel:  bun.BaseModel{},
				Id:         0,
				Migration0: "migration0",
			}
			_, err := tx.NewCreateTable().Model(testEntity0).Exec(ctx)
			if err != nil {
				logTest.Errorf("Failed table creation")
				return gerrors.WrapPrefix(err, "failed table creation", 0)
			}
			_, err = tx.NewInsert().Model(testEntity0).Exec(ctx)
			if err != nil {
				logTest.Errorf("Failed data insertion")
				return gerrors.WrapPrefix(err, "failed insertion", 0)
			}
			return nil
		})
		return err
	}, Down: func(ctx context.Context, db *bun.DB) error { return nil }}
	migrations.Add(migration0)
	migration1 := migrate.Migration{Name: "00000000000001", Comment: "second_migration", Up: func(ctx context.Context, db *bun.DB) error {
		logTest.Info("second_migration")
		err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			testEntity1 := &TestEntity1{
				BaseModel:  bun.BaseModel{},
				Id:         0,
				Migration0: "migration0",
				Migration1: "migration1",
			}
			_, err := tx.Exec("ALTER TABLE test_entity ADD COLUMN migration1 text")
			if err != nil {
				logTest.Errorf("Failed adding column")
				return gerrors.WrapPrefix(err, "failed insertion", 0)
			}
			if fail {
				logTest.Infof("Fail migration manually")
				return errors.New("fail migration manually")
			}
			_, err = tx.NewUpdate().Model(testEntity1).Column("migration1").Where("id=0").Exec(ctx)
			if err != nil {
				logTest.Errorf("Failed data updating")
				return gerrors.WrapPrefix(err, "failed data updating", 0)
			}
			return nil
		})
		return err
	}, Down: func(ctx context.Context, db *bun.DB) error { return nil }}
	migrations.Add(migration1)
	migration2 := migrate.Migration{Name: "00000000000002", Comment: "third_migration", Up: func(ctx context.Context, db *bun.DB) error {
		logTest.Info("third_migration")
		err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			testEntity2 := &TestEntity2{
				BaseModel:  bun.BaseModel{},
				Id:         0,
				Migration0: "migration0",
				Migration1: "migration1",
				Migration2: "migration2",
			}
			_, err := tx.Exec("ALTER TABLE test_entity ADD COLUMN migration2 text")
			if err != nil {
				logTest.Errorf("Failed adding column")
				return gerrors.WrapPrefix(err, "failed adding column", 0)
			}
			_, err = tx.NewUpdate().Model(testEntity2).Column("migration2").Where("id=0").Exec(ctx)
			if err != nil {
				logTest.Errorf("Failed data updating")
				return gerrors.WrapPrefix(err, "failed data updating", 0)
			}
			return nil
		})
		return err
	}, Down: func(ctx context.Context, db *bun.DB) error { return nil }}
	migrations.Add(migration2)

	//It is necessary to avoid fix migration17. Otherwise, test will fail
	migrationFix17 := migrate.Migration{Name: "00000000000017", Comment: "fix17_migration", Up: func(ctx context.Context, db *bun.DB) error {
		err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			logTest.Info("fix17_migration")
			return nil
		})
		return err
	}, Down: func(ctx context.Context, db *bun.DB) error { return nil }}
	migrations.Add(migrationFix17)
	//It is necessary to avoid fix migration24. Otherwise, test will fail
	migrationFix24 := migrate.Migration{Name: "00000000000024", Comment: "fix24_migration", Up: func(ctx context.Context, db *bun.DB) error {
		logTest.Info("fix24_migration")
		err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			return nil
		})
		return err
	}, Down: func(ctx context.Context, db *bun.DB) error { return nil }}
	migrations.Add(migrationFix24)
	return migrations
}
