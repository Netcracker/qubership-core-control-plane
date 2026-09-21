package db

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/tlsmode"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	dbaasbase "github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3"
	"github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3/model/rest"
	pgdbaas "github.com/netcracker/qubership-core-lib-go-dbaas-postgres-client/v4"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func (p *DefaultDBProvider) NewDB(subscribers ...func(db PgDB, event ConnEvent, err error)) (PgDB, error) {
	connProps, err := p.serviceDB.GetConnectionProperties(context.Background())
	if err != nil {
		log.Errorf("DefaultDBProvider failed to get service db connection properties: %v", err)
		return nil, err
	}
	pgOpts, err := pgx.ParseConfig(connProps.Url)
	if err != nil {
		log.Errorf("DefaultDBProvider failed to build listener pg Options: %v", err)
		return nil, err
	}
	pgOpts.Password = connProps.Password

	var tlsConfig *tls.Config
	if tlsmode.GetMode() == tlsmode.Preferred {
		tlsConfig = pgOpts.TLSConfig
	}

	connector := pgdriver.NewConnector(
		pgdriver.WithAddr(pgOpts.Host+":"+strconv.Itoa(int(pgOpts.Port))),
		pgdriver.WithUser(connProps.Username),
		pgdriver.WithPassword(connProps.Password),
		pgdriver.WithDatabase(pgOpts.Database),
		pgdriver.WithTLSConfig(tlsConfig),
	)

	passwdResetWrapper := pgDbWithPasswordReset{dbMux: &sync.RWMutex{}, terminated: false, dbaasClient: p.dbPool, config: connector, subscribers: subscribers}
	passwdResetWrapper.initDB()
	return &passwdResetWrapper, nil
}

type PgDB interface {
	Get() *bun.DB
	Close() error
}

type pgDbWithPasswordReset struct {
	dbMux      *sync.RWMutex
	pgDB       *bun.DB
	ticker     *time.Ticker
	terminated bool

	dbaasClient dbaasbase.DbaaSClient
	config      *pgdriver.Connector

	subscribers []func(db PgDB, event ConnEvent, err error)
}

func (db *pgDbWithPasswordReset) Get() *bun.DB {
	db.dbMux.RLock()
	defer db.dbMux.RUnlock()

	return db.pgDB
}

func (db *pgDbWithPasswordReset) initDB() {
	sqlDB := sql.OpenDB(db.config)
	db.pgDB = bun.NewDB(sqlDB, pgdialect.New())
	db.pgDB.SetMaxOpenConns(2)
	db.pgDB.SetConnMaxLifetime(0)
	go db.watchPasswordResets()
	// notify subs that db initialized
	go db.notifyAllSubscribers(context.Background(), Initialized, nil)
}

func (db *pgDbWithPasswordReset) watchPasswordResets() {
	db.ticker = time.NewTicker(30 * time.Second)
	for {
		if db.terminated {
			return
		}
		select {
		case <-db.ticker.C:
			if err := db.watchPasswordResetsWithRecovery(); err != nil {
				go db.notifyAllSubscribers(context.Background(), Error, err)
			}
		}
	}
}

func (db *pgDbWithPasswordReset) watchPasswordResetsWithRecovery() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("db: recovered panic in watching db password: %v", r))
			log.Errorf("%v", err)
		}
	}()

	var isValid bool
	if isValid, err = db.isPasswordValid(); !isValid && err == nil {
		err = db.getNewPasswordAndReconnect()
		if err != nil {
			log.Errorf("Error in getting new password and reconnecting to db: %v", err)
		}
	} else if err != nil {
		log.Errorf("Error during db password validation: %v", err)
	}
	return err
}

func (db *pgDbWithPasswordReset) isPasswordValid() (bool, error) {
	if _, err := db.pgDB.Exec("SELECT 1;"); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			pgErrCode := pgErr.Code // Code: the SQLSTATE code for the error
			return strings.Compare(pgErrCode, "28P01") != 0, nil
		}
		return false, err
	}
	return true, nil
}

func (db *pgDbWithPasswordReset) getNewPasswordAndReconnect() error {
	ctx := context.Background()
	newPassword, err := db.getPasswordFromDbaas(ctx)
	if err != nil {
		log.ErrorC(ctx, "Failed to get new password from dbaas: %v", err)
		return err
	}
	config := db.config.Config()
	config.Password = newPassword

	db.reconnect()
	go db.notifyAllSubscribers(ctx, PasswordReset, nil)
	return err
}

func (db *pgDbWithPasswordReset) notifyAllSubscribers(ctx context.Context, event ConnEvent, err error) {
	for _, sub := range db.subscribers {
		db.notifySubscriber(ctx, event, err, sub)
	}
}

func (db *pgDbWithPasswordReset) notifySubscriber(ctx context.Context, event ConnEvent, err error, callback func(db PgDB, event ConnEvent, err error)) {
	defer func() {
		if r := recover(); r != nil {
			log.ErrorC(ctx, "Recovered panic in password reset callback: %v", r)
		}
	}()
	callback(db, event, err)
}

func (db *pgDbWithPasswordReset) reconnect() {
	db.dbMux.Lock()
	defer db.dbMux.Unlock()

	if db.pgDB != nil {
		_ = db.pgDB.Close()
	}
	sqlDB := sql.OpenDB(db.config)
	db.pgDB = bun.NewDB(sqlDB, pgdialect.New())
}

func (db *pgDbWithPasswordReset) getPasswordFromDbaas(ctx context.Context) (string, error) {
	params := rest.BaseDbParams{}

	newConnection, dbErr := db.dbaasClient.GetConnection(ctx, pgdbaas.DB_TYPE, createControlPlaneServiceClassifier(ctx), params)
	if dbErr != nil {
		log.ErrorC(ctx, "Can't update connection with dbaasClient: %v", dbErr)
		return "", dbErr
	}
	if newPassword, ok := newConnection["password"]; ok {
		return newPassword.(string), nil
	}
	return "", errors.New("db: connection string doesn't contain password filed")
}

func (db *pgDbWithPasswordReset) Close() error {
	db.dbMux.Lock()
	defer db.dbMux.Unlock()

	db.terminated = true

	if db.ticker != nil {
		db.ticker.Stop()
	}

	if db.pgDB != nil {
		return db.pgDB.Close()
	}
	return nil
}
