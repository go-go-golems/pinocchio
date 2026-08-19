package serverkit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-go-golems/pinocchio/pkg/testsupport/mysqltest"
)

func TestMain(m *testing.M) {
	var instance *mysqltest.Instance
	if os.Getenv("PINOCCHIO_MYSQL_TESTCONTAINERS") == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		var err error
		instance, err = mysqltest.Start(ctx)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "start disposable MySQL: %v\n", err)
			os.Exit(1)
		}
		if err := os.Setenv("PINOCCHIO_MYSQL_TURNS_DSN", instance.AppDSN); err != nil {
			fmt.Fprintf(os.Stderr, "set application MySQL DSN: %v\n", err)
			os.Exit(1)
		}
		if err := os.Setenv("SESSIONSTREAM_MYSQL_DSN", instance.AppDSN); err != nil {
			fmt.Fprintf(os.Stderr, "set sessionstream MySQL DSN: %v\n", err)
			os.Exit(1)
		}
	}

	code := m.Run()
	if instance != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := instance.Close(ctx); err != nil && code == 0 {
			fmt.Fprintf(os.Stderr, "terminate disposable MySQL: %v\n", err)
			code = 1
		}
		cancel()
	}
	os.Exit(code)
}
