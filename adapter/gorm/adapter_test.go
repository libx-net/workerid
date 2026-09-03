package gormadapter

import (
	"context"
	"testing"

	"libx.net/workerid"
)

func TestNewClient_Nil(t *testing.T) {
	if NewClient(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestNewGenerator_NilDB(t *testing.T) {
	_, err := NewGenerator(nil, workerid.PostgresDialect(), "c")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInitializeCluster_NilClient(t *testing.T) {
	err := InitializeCluster(context.Background(), nil, workerid.PostgresDialect(), "c")
	if err == nil {
		t.Fatal("expected error")
	}
}
