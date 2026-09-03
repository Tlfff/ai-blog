package consumer

import (
	"github.com/google/wire"
)

var ProviderConsumerSet = wire.NewSet(
	NewConsumer,
)
