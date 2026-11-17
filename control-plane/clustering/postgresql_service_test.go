package clustering

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/db"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"testing"
	"time"
)

// Test error cases
// Required to suppress coverage requirement

func TestPostgreSqlService_NewPostgreSqlService(t *testing.T) {
	service := NewPostgreSqlService(nil)
	assert.NotNil(t, service)
}

func TestPostgreSqlService_CreateElectionTable(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.CreateElectionTable(&conn)
	})
}

func TestPostgreSqlService_InsertRecordOutdated(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.InsertRecordOutdated(&conn, &MasterMetadata{})
	})
}

func TestPostgreSqlService_InsertRecord(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.InsertRecord(&conn, &MasterMetadata{})
	})
}

func TestPostgreSqlService_DeleteAllRecords(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.DeleteAllRecords(&conn)
	})
}

func TestPostgreSqlService_DeleteNotInternalRecords(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.DeleteNotInternalRecords(&conn)
	})
}

func TestPostgreSqlService_UpdateMasterRecord(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.UpdateMasterRecord(&conn, &MasterMetadata{})
	})
}

func TestPostgreSqlService_ShiftSyncClock(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.ShiftSyncClock(&conn, 5*time.Second)
	})
}

func TestPostgreSqlService_ResetSyncClock(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.ResetSyncClock(&conn, "internal")
	})
}

func TestPostgreSqlService_GetMaster(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.GetMaster(&conn)
	})
}

func TestPostgreSqlService_Count(t *testing.T) {
	service := &PostgreSqlService{}
	conn := createTestConnection(t)
	assert.Panics(t, func() {
		service.Count(&conn)
	})
}

func TestPostgreSqlService_Conn(t *testing.T) {
	service := &PostgreSqlService{&DbProviderStub{DbProviderStubError}}
	_, err := service.Conn()
	assert.NotNil(t, err)
}

func TestPostgreSqlService_WithTx(t *testing.T) {
	service := &PostgreSqlService{&DbProviderStub{DbProviderStubError}}
	err := service.WithTx(func(conn *bun.Conn) error {
		return nil
	})
	assert.NotNil(t, err)
}

func createTestConnection(t *testing.T) bun.Conn {
	t.Helper()
	// Fail connection establishment immediately; a real dial timeout is irrelevant
	// to the nil-connection error cases exercised by these tests.
	sqldb := sql.OpenDB(unavailableTestDriver{})
	bunDB := bun.NewDB(sqldb, pgdialect.New())
	t.Cleanup(func() { assert.NoError(t, bunDB.Close()) })
	conn, err := bunDB.Conn(context.Background())
	assert.ErrorIs(t, err, DbProviderStubError)
	return conn
}

type unavailableTestDriver struct{}

func (unavailableTestDriver) Connect(context.Context) (driver.Conn, error) {
	return nil, DbProviderStubError
}
func (d unavailableTestDriver) Driver() driver.Driver { return d }
func (d unavailableTestDriver) Open(string) (driver.Conn, error) {
	return d.Connect(context.Background())
}

var DbProviderStubError = fmt.Errorf("DbProviderStub invoked error")

type DbProviderStub struct {
	throwError error
}

func (p *DbProviderStub) GetConn(_ context.Context) (*bun.Conn, error) {
	return nil, p.throwError
}

func (p *DbProviderStub) GetDB(_ context.Context) (*bun.DB, error) {
	return nil, p.throwError
}

func (p *DbProviderStub) NewDB(_ ...func(db db.PgDB, event db.ConnEvent, err error)) (db.PgDB, error) {
	return nil, nil
}

func (p *DbProviderStub) Listen(_ string, _ func(), _ func(payload string)) (db.PersistentStorageListener, error) {
	return nil, nil
}

type BlockingDbProviderStub struct {
	DbProviderStub
}

func (p *BlockingDbProviderStub) GetConn(ctx context.Context) (*bun.Conn, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestPostgreSqlService_Conn_PingTimeout(t *testing.T) {
	original := dbCallTimeout
	dbCallTimeout = 50 * time.Millisecond
	defer func() { dbCallTimeout = original }()

	service := NewPostgreSqlService(&BlockingDbProviderStub{})

	conn, err := service.Conn()

	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
