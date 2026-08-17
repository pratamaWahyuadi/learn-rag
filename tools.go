//go:build tools

// Package tools pins the module dependencies so that `go mod tidy` keeps them
// in go.mod/go.sum even though they are only imported from generated code,
// provider adapters, and build tooling in later phases.
package tools

import (
	_ "github.com/aws/aws-sdk-go-v2/config"
	_ "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	_ "github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/gin-gonic/gin"
	_ "github.com/google/uuid"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/pgvector/pgvector-go"
	_ "github.com/pressly/goose/v3"
)
