package pgpool

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgPool struct {
	masterPool  *pgxpool.Pool
	replicaPool *pgxpool.Pool
	mu          sync.RWMutex
}

type PConn struct {
	conn      *pgxpool.Conn
	pool      *pgxpool.Pool
	pgpool    *PgPool
	isReplica bool
}

func NewPgPool(connStrings []string, poolSize int) (*PgPool, error) {
	p := &PgPool{}

	// Настройка пула для мастера
	if len(connStrings) > 0 {
		config, err := pgxpool.ParseConfig(connStrings[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse master config: %w", err)
		}
		config.MaxConns = int32(poolSize)
		config.MinConns = int32(poolSize / 2)
		config.MaxConnLifetime = 1 * time.Hour
		config.MaxConnIdleTime = 30 * time.Minute

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		masterPool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create master pool: %w", err)
		}

		// Проверка роли
		var isRecovery bool
		err = masterPool.QueryRow(ctx, "SELECT pg_is_in_recovery()").Scan(&isRecovery)
		if err == nil && !isRecovery {
			p.masterPool = masterPool
			log.Printf("✓ Master pool created: %s", connStrings[0])
		} else {
			masterPool.Close()
			log.Printf("⚠ Connection is not a master: %s", connStrings[0])
		}
	}

	// Настройка пула для реплики
	if len(connStrings) > 1 {
		config, err := pgxpool.ParseConfig(connStrings[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse replica config: %w", err)
		}
		config.MaxConns = int32(poolSize)
		config.MinConns = int32(poolSize / 2)
		config.MaxConnLifetime = 1 * time.Hour
		config.MaxConnIdleTime = 30 * time.Minute

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		replicaPool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			log.Printf("⚠ Failed to create replica pool: %v", err)
		} else {
			// Проверка роли
			var isRecovery bool
			err = replicaPool.QueryRow(ctx, "SELECT pg_is_in_recovery()").Scan(&isRecovery)
			if err == nil && isRecovery {
				p.replicaPool = replicaPool
				log.Printf("✓ Replica pool created: %s", connStrings[1])
			} else {
				replicaPool.Close()
				log.Printf("⚠ Connection is not a replica: %s", connStrings[1])
			}
		}
	}

	if p.masterPool == nil && p.replicaPool == nil {
		return nil, errors.New("no valid database connections available")
	}

	return p, nil
}

func (p *PgPool) Acquire(ctx context.Context, readOnly bool) (*PConn, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Для операций чтения пытаемся использовать реплику
	if readOnly && p.replicaPool != nil {
		conn, err := p.replicaPool.Acquire(ctx)
		if err == nil {
			return &PConn{
				conn:      conn,
				pool:      p.replicaPool,
				pgpool:    p,
				isReplica: true,
			}, nil
		}
		log.Printf("Failed to acquire replica connection: %v", err)
	}

	// Для операций записи или если реплика недоступна - используем мастер
	if p.masterPool != nil {
		conn, err := p.masterPool.Acquire(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire master connection: %w", err)
		}

		if readOnly && p.replicaPool == nil {
			log.Println("No replica available, using MASTER for READ operation")
		}

		return &PConn{
			conn:      conn,
			pool:      p.masterPool,
			pgpool:    p,
			isReplica: false,
		}, nil
	}

	return nil, errors.New("no available database connections")
}

func (pc *PConn) Release() {
	if pc.conn != nil {
		pc.conn.Release()
	}
}

func (pc *PConn) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return pc.conn.QueryRow(ctx, sql, args...)
}

func (pc *PConn) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return pc.conn.Query(ctx, sql, args...)
}

func (pc *PConn) Exec(ctx context.Context, sql string, args ...interface{}) error {
	_, err := pc.conn.Exec(ctx, sql, args...)
	return err
}

func (pc *PConn) Begin(ctx context.Context) (pgx.Tx, error) {
	return pc.conn.Begin(ctx)
}

func (p *PgPool) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Проверка мастера
	if p.masterPool != nil {
		if err := p.masterPool.Ping(ctx); err != nil {
			log.Printf("Master health check failed: %v", err)
			// Можно добавить логику переподключения
		}
	}

	// Проверка реплики
	if p.replicaPool != nil {
		if err := p.replicaPool.Ping(ctx); err != nil {
			log.Printf("Replica health check failed: %v", err)
		}
	}

	return nil
}

func (p *PgPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.masterPool != nil {
		p.masterPool.Close()
		log.Println("Master pool closed")
	}
	if p.replicaPool != nil {
		p.replicaPool.Close()
		log.Println("Replica pool closed")
	}
}
