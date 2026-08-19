// Package mysqltest provides a disposable MySQL integration-test environment.
// It is intended for _test.go consumers and must not be used by production
// runtime code.
package mysqltest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
)

const (
	image             = "mysql:8.4.8"
	bootstrapDatabase = "pinocchio_bootstrap"
	appUser           = "pinocchio_app"
)

// Instance is a disposable MySQL server and the two credentials used by the
// integration tests. AppDSN is the only DSN that should be passed to the
// adapter under test. AdminDSN is reserved for test setup and fault injection.
type Instance struct {
	Container testcontainers.Container
	AppDSN    string
	AdminDSN  string
	Database  string
}

// Start starts a fresh MySQL container, creates a random database and creates
// a database-scoped application user. The container has no volume or fixed
// host port and is safe to terminate after the package test completes.
func Start(ctx context.Context) (*Instance, error) {
	if ctx == nil {
		return nil, fmt.Errorf("mysqltest: context is nil")
	}
	container, err := tcmysql.Run(
		ctx,
		image,
		tcmysql.WithDatabase(bootstrapDatabase),
		tcmysql.WithUsername("root"),
		tcmysql.WithPassword("test-root-password"),
	)
	if err != nil {
		return nil, fmt.Errorf("mysqltest: start %s: %w", image, err)
	}
	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			_ = container.Terminate(context.Background())
		}
	}()

	baseDSN, err := container.ConnectionString(ctx, "parseTime=true")
	if err != nil {
		return nil, fmt.Errorf("mysqltest: get connection string: %w", err)
	}
	baseConfig, err := mysql.ParseDSN(baseDSN)
	if err != nil {
		return nil, fmt.Errorf("mysqltest: parse connection string: %w", err)
	}

	database, err := randomIdentifier("pinocchio_test_")
	if err != nil {
		return nil, err
	}
	password, err := randomSecret(24)
	if err != nil {
		return nil, err
	}

	adminConfig := *baseConfig
	adminConfig.DBName = bootstrapDatabase
	adminDSN := adminConfig.FormatDSN()
	adminDB, err := sql.Open("mysql", adminDSN)
	if err != nil {
		return nil, fmt.Errorf("mysqltest: open admin connection: %w", err)
	}
	defer func() { _ = adminDB.Close() }()
	if err := adminDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("mysqltest: ping admin connection: %w", err)
	}

	if _, err := adminDB.ExecContext(ctx, "CREATE DATABASE "+quoteIdentifier(database)); err != nil {
		return nil, fmt.Errorf("mysqltest: create database %q: %w", database, err)
	}
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf(
		"CREATE USER '%s'@'%%' IDENTIFIED BY '%s'",
		appUser,
		escapeSQLString(password),
	)); err != nil {
		return nil, fmt.Errorf("mysqltest: create application user: %w", err)
	}
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf(
		"GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX, DROP ON %s.* TO '%s'@'%%'",
		quoteIdentifier(database),
		appUser,
	)); err != nil {
		return nil, fmt.Errorf("mysqltest: grant application user: %w", err)
	}

	adminConfig.DBName = database
	appConfig := adminConfig
	appConfig.User = appUser
	appConfig.Passwd = password
	instance := &Instance{
		Container: container,
		AdminDSN:  adminConfig.FormatDSN(),
		AppDSN:    appConfig.FormatDSN(),
		Database:  database,
	}
	cleanupOnError = false
	return instance, nil
}

// Close terminates the disposable container. It is safe to call more than
// once; callers should pass a bounded cleanup context.
func (i *Instance) Close(ctx context.Context) error {
	if i == nil || i.Container == nil {
		return nil
	}
	if ctx == nil {
		return fmt.Errorf("mysqltest: cleanup context is nil")
	}
	return i.Container.Terminate(ctx)
}

func randomIdentifier(prefix string) (string, error) {
	secret, err := randomSecret(8)
	if err != nil {
		return "", fmt.Errorf("mysqltest: generate identifier: %w", err)
	}
	return prefix + strings.ToLower(secret), nil
}

func randomSecret(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	if length <= 0 {
		return "", fmt.Errorf("mysqltest: invalid secret length %d", length)
	}
	buf := make([]byte, length)
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i, b := range raw {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

func quoteIdentifier(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func escapeSQLString(value string) string {
	return strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(value)
}
