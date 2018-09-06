package consensus

import (
	"time"

	"github.com/wunderlist/ttlcache"
)

// internal caches for types unmarshalled from raw proto message,
// the main purpose of these caches is to minimize the cost of
// frequent unmarshalling of the same type.
var (
	ballotCache      *ttlcache.Cache
	prepareCache     *ttlcache.Cache
	confirmCache     *ttlcache.Cache
	externalizeCache *ttlcache.Cache
	nominationCache  *ttlcache.Cache
)

func init() {
	ballotCache = ttlcache.NewCache(time.Minute)
	prepareCache = ttlcache.NewCache(time.Minute)
	confirmCache = ttlcache.NewCache(time.Minute)
	externalizeCache = ttlcache.NewCache(time.Minute)
	nominationCache = ttlcache.NewCache(time.Minute)
}
