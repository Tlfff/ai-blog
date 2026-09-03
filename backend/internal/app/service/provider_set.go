package service

import (
	"github.com/google/wire"
)

// ServiceProviderAppSet is service providers.
var ServiceProviderAppSet = wire.NewSet(
	NewBlogServer,
	NewBookServer,
)

// ServiceGrpcProviderAppSet is service providers.
var ServiceGrpcProviderAppSet = wire.NewSet(
	NewGrpcBlogServer,
	NewGrpcBookServer,
)
