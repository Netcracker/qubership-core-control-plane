package clustering

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/db"
	"github.com/uptrace/bun"
)

var dbCallTimeout = 5 * time.Second

var ctx = context.Background()

type SqlService interface {
	CreateElectionTable(cnn *bun.Conn) (sql.Result, error)
	InsertRecordOutdated(cnn *bun.Conn, record *MasterMetadata) (int64, error)
	InsertRecord(cnn *bun.Conn, record *MasterMetadata) (int64, error)
	DeleteAllRecords(cnn *bun.Conn) (int64, error)
	DeleteNotInternalRecords(cnn *bun.Conn) (int64, error)
	UpdateMasterRecord(cnn *bun.Conn, record *MasterMetadata) (int64, error)
	ShiftSyncClock(cnn *bun.Conn, d time.Duration) (int64, error)
	ResetSyncClock(cnn *bun.Conn, recordName string) (int64, error)
	GetMaster(cnn *bun.Conn) (*MasterMetadata, error)
	Count(cnn *bun.Conn) (int, error)
	WithTx(f func(conn *bun.Conn) error) error
	Conn() (*bun.Conn, error)
}

type PostgreSqlService struct {
	dbProvider db.DBProvider
}

func newDBContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), dbCallTimeout)
}

func NewPostgreSqlService(dbProvider db.DBProvider) *PostgreSqlService {
	return &PostgreSqlService{
		dbProvider: dbProvider,
	}
}

func (p *PostgreSqlService) CreateElectionTable(cnn *bun.Conn) (sql.Result, error) {
	return cnn.NewCreateTable().Model(&MasterMetadata{}).IfNotExists().Exec(ctx)
}

func (p *PostgreSqlService) InsertRecordOutdated(cnn *bun.Conn, record *MasterMetadata) (int64, error) {
	res, err := cnn.NewInsert().Model(record).Value("sync_clock", "now() - interval '1 hour'").Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (p *PostgreSqlService) InsertRecord(cnn *bun.Conn, record *MasterMetadata) (int64, error) {
	res, err := cnn.NewInsert().
		Model(record).
		Value("sync_clock", "now() + interval '60 second'").
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (p *PostgreSqlService) DeleteAllRecords(cnn *bun.Conn) (int64, error) {
	res, err := cnn.NewDelete().Model(&MasterMetadata{}).Where("id = id").Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (p *PostgreSqlService) DeleteNotInternalRecords(cnn *bun.Conn) (int64, error) {
	res, err := cnn.NewDelete().Model(&MasterMetadata{}).Where("name <> ?", "internal").Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (p *PostgreSqlService) UpdateMasterRecord(cnn *bun.Conn, record *MasterMetadata) (int64, error) {
	res, err := cnn.NewUpdate().Model(record).
		Value("sync_clock", "now() + interval '60 second'").
		Where("sync_clock < now() OR namespace != ?", record.Namespace).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (p *PostgreSqlService) ShiftSyncClock(cnn *bun.Conn, d time.Duration) (int64, error) {
	query := fmt.Sprintf("UPDATE %s SET sync_clock = now() + interval '%.0f second';", ElectionTableName, d.Seconds())
	res, err := cnn.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (p *PostgreSqlService) ResetSyncClock(cnn *bun.Conn, recordName string) (int64, error) {
	query := fmt.Sprintf("UPDATE %s SET sync_clock = ? WHERE name = ?;", ElectionTableName)
	res, err := cnn.ExecContext(ctx, query, "now()", recordName)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (p *PostgreSqlService) GetMaster(cnn *bun.Conn) (*MasterMetadata, error) {
	var res MasterMetadata
	err := cnn.NewSelect().Model(&res).OrderExpr("id ASC").Limit(1).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (p *PostgreSqlService) Count(cnn *bun.Conn) (int, error) {
	return cnn.NewSelect().Model(&MasterMetadata{}).Count(ctx)
}

func (p *PostgreSqlService) Conn() (*bun.Conn, error) {
	ctx, cancel := newDBContext()
	defer cancel()
	return p.dbProvider.GetConn(ctx)
}

func (p *PostgreSqlService) WithTx(f func(conn *bun.Conn) error) error {
	cnn, err := p.Conn()
	if err != nil {
		return err
	}
	defer cnn.Close()

	tx, err := cnn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := f(cnn); err != nil {
		return err
	}
	return tx.Commit()
}
