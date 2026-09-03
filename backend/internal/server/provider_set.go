package server

import (
	"github.com/google/wire"
)

// ProviderServerSet is server providers.
var ProviderServerSet = wire.NewSet(NewHTTPServer)
var ProviderGrpcServerSet = wire.NewSet(NewGrpcServer)
