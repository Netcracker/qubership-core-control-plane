package clustering

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/db"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"net"
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
	ctx := context.Background()
	dsn := NewTestPgOptions()
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	db := bun.NewDB(sqldb, pgdialect.New())
	conn, err := db.Conn(ctx)
	assert.NotNil(t, err)
	return conn
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

func NewTestPgOptions() string {
	// Free port is required to guarantee an error on pg.Conn functions
	return fmt.Sprintf("postgresql://localhost:%d", findFreePort())
}

func findFreePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

type BlockingDbProviderStub struct {
	DbProviderStub
}

func (p *BlockingDbProviderStub) GetConn(ctx context.Context) (*bun.Conn, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestPostgreSqlService_Conn_PingTimeout(t *testing.T) {
	service := NewPostgreSqlService(&BlockingDbProviderStub{})

	conn, err := service.Conn()

	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
