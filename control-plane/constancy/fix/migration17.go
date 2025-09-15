package fix

import (
	"context"
	"database/sql"
	"fmt"
	gerrors "github.com/go-errors/errors"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"strconv"
	"strings"
	"time"
)

const (
	migrationTableName    = "bun_migrations"
	cockroachdbErrorMatch = `at or near "lock"`
	yugabytedbErrorMatch  = `lock mode not supported yet`
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("fix-migration")
}

func Migration17(db *bun.DB, ctx context.Context, migrations *migrate.Migrations) error {
	exist, err := checkMigration17Exist(db)
	if err != nil {
		return gerrors.Wrap(err, 0)
	}
	if !exist {
		logger.Warnf("Detected absence of migration # 17. Applying...")
		tx, err := begin(db)
		if err != nil {
			return gerrors.Wrap(err, 0)
		}
		defer func() {
			_ = tx.Rollback()
		}()
		if err := applyMigration17(db, migrations, ctx); err != nil {
			return gerrors.Wrap(err, 0)
		}
		if err := writeMigration17ToTable(tx, ctx); err != nil {
			return gerrors.Wrap(err, 0)
		}
		_ = tx.Commit()
		logger.Warnf("Migration #17 has been applied")
	}
	return nil
}

func writeMigration17ToTable(tx *bun.Tx, ctx context.Context) error {
	var migrationRecords []migrationRecord
	err := tx.NewSelect().Model(&migrationRecords).OrderExpr("id DESC").Scan(ctx)
	if err != nil {
		return gerrors.Wrap(err, 0)
	}
	var rec17Id int32
	var maxId int32
	for _, rec := range migrationRecords {
		if maxId < rec.Id {
			maxId = rec.Id
		}
		migrationNum, err := strconv.ParseInt(rec.Name, 10, 0)
		if err != nil {
			logger.Errorf("Wrong migration name %+v", err.Error())
			return err
		}
		if migrationNum > 17 {
			if migrationNum == 18 {
				rec17Id = rec.Id
			}
			_, err := tx.NewUpdate().Model(&rec).WherePK().Set("id = ?", rec.Id+1).Exec(ctx)
			if err != nil {
				return gerrors.Wrap(err, 0)
			}
			logger.Debugf("%v. Updated.", rec)
		}
	}
	rec17 := migrationRecord{
		Id:         rec17Id,
		Name:       "00000000000017",
		GroupId:    0,
		MigratedAt: time.Now(),
	}
	_, err = tx.NewInsert().Model(&rec17).Where("id = ?", rec17Id).Exec(ctx)
	if err != nil {
		return gerrors.Wrap(err, 0)
	}
	logger.Debug("%v. Inserted.", rec17)
	for actualSeqValue := int32(0); maxId+1 > actualSeqValue; {
		err = tx.QueryRow(`SELECT nextval('bun_migrations_id_seq')`, bun.Safe(migrationTableName)).
			Scan(&actualSeqValue)
		if err != nil {
			return gerrors.Wrap(err, 0)
		}
		logger.Debug("Got value from bun_migrations_id_seq: '%d', max(id) from bun_migrations: '%d'", actualSeqValue, maxId+1)
	}
	return nil
}

type migrationRecord struct {
	bun.BaseModel `bun:"table:gopg_migrations"`
	Id            int32
	Name          string
	GroupId       int64
	MigratedAt    time.Time `bun:"migrated_at"`
}

func (r migrationRecord) String() string {
	return fmt.Sprintf("MigrationRecord{id=%d,Name=%s,Group=%d,migratedAt=%s}", r.Id, r.Name, r.GroupId, r.MigratedAt.Format(time.RFC3339))
}

func applyMigration17(tx *bun.DB, migrations *migrate.Migrations, ctx context.Context) error {
	for _, migration := range migrations.Sorted() {
		if migration.Name == "00000000000017" {
			if err := migration.Up(ctx, tx, nil); err != nil {
				return gerrors.Wrap(err, 0)
			}
		}
	}
	return nil
}

func checkMigration17Exist(db *bun.DB) (bool, error) {
	var name string
	err := db.QueryRow(`
		SELECT name FROM ? WHERE name = '00000000000017' LIMIT 1
	`, bun.Safe(migrationTableName)).Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return name != "", nil
}

func begin(db *bun.DB) (*bun.Tx, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	// If there is an error setting this, rollback the transaction and don't bother doing it
	// because Postgres < 9.6 doesn't support this
	_, err = tx.Exec("SET idle_in_transaction_session_timeout = 0")
	if err != nil {
		_ = tx.Rollback()

		tx, err = db.Begin()
		if err != nil {
			return nil, err
		}
	}
	// If there is an error setting this, rollback the transaction and don't bother doing it
	// because neither CockroachDB nor Yugabyte support it
	_, err = tx.Exec("LOCK TABLE ?", bun.Safe(migrationTableName))
	if err != nil {
		_ = tx.Rollback()

		if !strings.Contains(err.Error(), cockroachdbErrorMatch) && !strings.Contains(err.Error(), yugabytedbErrorMatch) {
			return nil, err
		}
		tx, err = db.Begin()
		if err != nil {
			return nil, err
		}
	}

	return &tx, nil
}
