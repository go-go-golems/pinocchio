package mysqltest

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestStartDisposableMySQL(t *testing.T) {
	if os.Getenv("PINOCCHIO_MYSQL_TESTCONTAINERS") != "1" {
		t.Skip("set PINOCCHIO_MYSQL_TESTCONTAINERS=1 to run disposable MySQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	instance, err := Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := instance.Close(cleanupCtx); err != nil {
			t.Errorf("close disposable MySQL: %v", err)
		}
	})
	if instance.AppDSN == instance.AdminDSN {
		t.Fatal("application and admin DSNs must be distinct")
	}
	appDB, err := sql.Open("mysql", instance.AppDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = appDB.Close() }()
	if err := appDB.PingContext(ctx); err != nil {
		t.Fatalf("ping application DSN: %v", err)
	}
	var currentUser string
	if err := appDB.QueryRowContext(ctx, "SELECT CURRENT_USER()").Scan(&currentUser); err != nil {
		t.Fatal(err)
	}
	if currentUser == "root@%" || currentUser == "root@localhost" {
		t.Fatalf("application DSN unexpectedly uses root: %s", currentUser)
	}
}
