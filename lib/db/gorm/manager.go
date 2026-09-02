package gormdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/dbresolver"
)

var ErrInstanceNotFound = errors.New("gorm instance not found")

const connectionMaxLifetime = 5 * time.Minute

type trackedPool struct {
	role string
	db   *sql.DB
}

type instance struct {
	db    *gorm.DB
	pools []trackedPool
}

type Manager struct {
	mu        sync.RWMutex
	instances map[string]*instance
	openPool  func(string) (*sql.DB, error)
}

func NewManager() *Manager {
	return &Manager{
		instances: make(map[string]*instance),
		openPool: func(dsn string) (*sql.DB, error) {
			return sql.Open("mysql", dsn)
		},
	}
}

func (m *Manager) InitAndRun(ctx context.Context, configs []Config, models ...interface{}) error {
	added := make([]string, 0, len(configs))
	for _, config := range configs {
		if err := m.addInstance(ctx, config, models...); err != nil {
			for i := len(added) - 1; i >= 0; i-- {
				_ = m.closeInstance(added[i])
			}
			return err
		}
		added = append(added, config.IndexName)
	}
	return nil
}

func (m *Manager) addInstance(ctx context.Context, config Config, models ...interface{}) error {
	if err := config.validate(); err != nil {
		return err
	}
	logger.Infof("init gorm data source {name:%s master:%s replicas:%d}",
		config.IndexName, redactDSN(buildDSN(config.Master)), len(config.Slaves))

	pools := make([]trackedPool, 0, 1+len(config.Slaves))
	closePools := func() {
		for i := len(pools) - 1; i >= 0; i-- {
			_ = pools[i].db.Close()
		}
	}

	masterPool, err := m.openConfiguredPool(ctx, config.Master, config)
	if err != nil {
		return fmt.Errorf("gorm instance %q master: %w", config.IndexName, err)
	}
	pools = append(pools, trackedPool{role: "master", db: masterPool})

	replicaDialectors := make([]gorm.Dialector, 0, len(config.Slaves))
	for i, slave := range config.Slaves {
		pool, openErr := m.openConfiguredPool(ctx, slave, config)
		if openErr != nil {
			closePools()
			return fmt.Errorf("gorm instance %q replica_%d: %w", config.IndexName, i, openErr)
		}
		pools = append(pools, trackedPool{role: fmt.Sprintf("replica_%d", i), db: pool})
		replicaDialectors = append(replicaDialectors, mysql.New(mysql.Config{
			Conn:                      pool,
			SkipInitializeWithVersion: true,
		}))
	}

	logMode := gormlogger.Silent
	if config.ShowSQL {
		logMode = gormlogger.Info
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      masterPool,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		NamingStrategy:                           schema.NamingStrategy{SingularTable: true},
		Logger:                                   gormlogger.Default.LogMode(logMode),
		DisableForeignKeyConstraintWhenMigrating: true,
		DisableAutomaticPing:                     true,
	})
	if err != nil {
		closePools()
		return fmt.Errorf("gorm instance %q open: %w", config.IndexName, err)
	}

	if config.InitFlag && len(models) > 0 {
		if err := db.WithContext(ctx).AutoMigrate(models...); err != nil {
			closePools()
			return fmt.Errorf("gorm instance %q auto migrate: %w", config.IndexName, err)
		}
	}
	if len(replicaDialectors) > 0 {
		if err := db.Use(dbresolver.Register(dbresolver.Config{Replicas: replicaDialectors})); err != nil {
			closePools()
			return fmt.Errorf("gorm instance %q register replicas: %w", config.IndexName, err)
		}
	}

	created := &instance{db: db, pools: pools}
	m.mu.Lock()
	previous := m.instances[config.IndexName]
	m.instances[config.IndexName] = created
	m.mu.Unlock()
	if previous != nil {
		_ = closeTrackedPools(previous.pools)
	}
	registerGormMetrics(config.IndexName, created)
	return nil
}

func (m *Manager) openConfiguredPool(ctx context.Context, info *DbInfo, config Config) (*sql.DB, error) {
	pool, err := m.openPool(buildDSN(info))
	if err != nil {
		return nil, err
	}
	pool.SetMaxIdleConns(config.MaxIdle)
	pool.SetMaxOpenConns(config.MaxOpen)
	pool.SetConnMaxLifetime(connectionMaxLifetime)
	finish := beginGormPingObserve(config.IndexName)
	err = pool.PingContext(ctx)
	finish(err)
	if err != nil {
		_ = pool.Close()
		return nil, err
	}
	return pool, nil
}

func (m *Manager) GetDB(name ...string) (*gorm.DB, error) {
	key := "default"
	if len(name) > 0 && name[0] != "" {
		key = name[0]
	}
	m.mu.RLock()
	entry := m.instances[key]
	m.mu.RUnlock()
	if entry == nil || entry.db == nil {
		return nil, fmt.Errorf("%w: %s", ErrInstanceNotFound, key)
	}
	return entry.db, nil
}

func (m *Manager) Transaction(ctx context.Context, name string, fn func(*gorm.DB) error) error {
	if fn == nil {
		return errors.New("gorm transaction callback is nil")
	}
	db, err := m.GetDB(name)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(fn)
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	instances := m.instances
	m.instances = make(map[string]*instance)
	m.mu.Unlock()

	var errs []error
	for name, entry := range instances {
		if err := closeTrackedPools(entry.pools); err != nil {
			errs = append(errs, fmt.Errorf("gorm instance %q close: %w", name, err))
		}
		unregisterGormMetrics(name)
	}
	return errors.Join(errs...)
}

func (m *Manager) closeInstance(name string) error {
	m.mu.Lock()
	entry := m.instances[name]
	delete(m.instances, name)
	m.mu.Unlock()
	if entry == nil {
		return nil
	}
	unregisterGormMetrics(name)
	return closeTrackedPools(entry.pools)
}

func closeTrackedPools(pools []trackedPool) error {
	var errs []error
	for i := len(pools) - 1; i >= 0; i-- {
		if pools[i].db != nil {
			if err := pools[i].db.Close(); err != nil {
				errs = append(errs, fmt.Errorf("%s pool: %w", pools[i].role, err))
			}
		}
	}
	return errors.Join(errs...)
}
