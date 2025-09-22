package db

import (
	"context"
	"github.com/uptrace/bun/driver/pgdriver"
	"time"
)

func (p *DefaultDBProvider) Listen(channel string, connectCallback func(), notificationCallback func(payload string)) (PersistentStorageListener, error) {
	postgresListener := &postgresListener{channel: channel, connectCallback: connectCallback, notificationCallback: notificationCallback}
	_, err := p.NewDB(postgresListener.processConnEvent) // returned PgDB will be saved by processConnEvent function
	if err != nil {
		log.Infof("Failed to initialize pg db listener: %v", err)
		return nil, err
	}
	return postgresListener, nil
}

//go:generate mockgen -source=listener.go -destination=../test/mock/db/stub_listener.go -package=mock_db
type PersistentStorageListener interface {
	Close()
}

type postgresListener struct {
	channel              string
	connectCallback      func()
	notificationCallback func(payload string)
	listener             *pgdriver.Listener
	db                   PgDB
	ctx                  context.Context
	cancelFunc           context.CancelFunc
}

func (l *postgresListener) processConnEvent(db PgDB, event ConnEvent, err error) {
	switch event {
	case Initialized:
		log.Info("Postgres DB to listen has been initialized")
		l.db = db
		l.startListening()
		break
	case PasswordReset:
		log.Info("Postgres DB password reset")
		l.resetListener()
		break
	case Error:
		log.Errorf("Error ConnEvent from postgres DB being listened: %v", err)
		l.resetListener()
		break
	default:
		log.Warnf("Unsupported postgres DB ConnEvent: %v", event)
		break
	}
}

func (l *postgresListener) resetListener() {
	l.Close()
	time.Sleep(1 * time.Second)
	l.startListening()
}

func (l *postgresListener) startListening() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered panic in pg listener loop: %v; Resetting listener...", r)
			l.resetListener()
		}
	}()

	log.Infof("Starting pg listener on channel %s", l.channel)
	l.ctx, l.cancelFunc = context.WithCancel(context.Background())
	l.listener = pgdriver.NewListener(l.db.Get())
	if err := l.listener.Listen(context.Background(), l.channel); err != nil {
		log.Errorf("Can't start listening postgres: %+v", err.Error())
		panic(err)
	}

	l.connectCallback()
	for {
		select {
		case notification := <-l.listener.CreateChannel():
			payload := notification.Payload
			log.Debugf("Received notification from pg on %s chan: %s", notification.Channel, payload)
			l.notificationCallback(payload)
		case <-l.ctx.Done():
			log.Infof("Terminated pg listener loop")
			return
		}
	}
}

func (l *postgresListener) execCallback(payload string, callback func(payload string)) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Listener on %s got panic recovered: %v", l.channel, r)
		}
	}()
	callback(payload)
}

func (l *postgresListener) Close() {
	log.Infof("Closing pg listener on channel %s", l.channel)
	l.cancelFunc()

	if l.listener != nil {
		if err := l.listener.Close(); err != nil {
			log.Warnf("Error in closing listener on %s: %v", l.channel, err)
		}
	}
	if l.db != nil {
		if err := l.db.Close; err != nil {
			log.Warnf("Error in closing pg DB: %v", l.channel, err)
		}
	}
}
