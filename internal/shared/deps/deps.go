// Package deps anchors the SPEC §2 dependency pins (E01-T01).
//
// `go mod tidy` drops requirements no package imports, so this package
// blank-imports one package per pinned module to retain the reproducible
// versions until real code imports them. It exports no API and executes no
// logic. Later epics delete entries here as production imports land; the last
// owner removes the package once tidy retains every pin through real imports.
package deps

import (
	_ "github.com/99designs/gqlgen/graphql"
	_ "github.com/casbin/casbin/v2"
	_ "github.com/dgraph-io/ristretto/v2"
	_ "github.com/go-co-op/gocron"
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/labstack/echo/v5"
	_ "github.com/nats-io/nats.go"
	_ "github.com/open-feature/go-sdk/openfeature"
	_ "github.com/pressly/goose/v3"
	_ "github.com/redis/go-redis/v9"
	_ "github.com/stretchr/testify/assert"
	_ "golang.org/x/oauth2"
	_ "google.golang.org/grpc"
	_ "google.golang.org/protobuf/proto"
	_ "gorm.io/gorm"
)
