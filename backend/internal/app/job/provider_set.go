package job

import (
	"github.com/google/wire"
)

var ProviderJobSet = wire.NewSet(
	NewJob,
)
