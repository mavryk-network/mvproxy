# MvProxy

MvProxy is a reverse proxy specifically designed for Mavryk nodes, offering a range of features that enhance node performance, security, and manageability.

## Features

- [x] Rate limit
- [x] Block IPs
- [x] Blocklist routes
- [x] Cache
- [x] CORS
- [x] GZIP
- [x] Metrics
- [x] Redis


## How to run

Make sure that your Mavryk Node is running and set its host address in the `mavryk_host` configuration.

If you want to test only MvProxy without a real Mavryk Node, you can simulate a Mavryk Node with our `flextesa.sh` script. Make sure that you have a docker.

```bash
./flexmasa.sh
```

If you want custom configurations, create a file named as `mvproxy.yaml` in the same directory of the binary. This file will be created if you don't create it.

Then, just [download the binary](https://github.com/mavryk-network/mvproxy/releases) and run it:
```bash
./mvproxy
```

Finally, test it with:
```bash
curl http://localhost:8080/chains/main/blocks/head/header
```

## Configuration

### Yaml File
Here a default `mvproxy.yaml` file:

```yaml
allow_routes:
    enabled: true
    values:
        - GET/chains/.*/blocks
        - GET/chains/.*/chain_id
        - GET/chains.*/checkpoint
        - GET/chains/.*/invalid_blocks
        - GET/chains.*/invalid_blocks.*
        - GET/chains/.*/is_bootstrapped
        - GET/chains.*/mempool/filter
        - GET/chains/.*/mempool/monitor_operations
        - GET/chains/.*/mempool/pending_operations
        - GET/config/network/user_activated_protocol_overrides
        - GET/config/network/user_activated_upgrades
        - GET/config/network/dal
        - GET/describe.*
        - GET/errors
        - GET/monitor.*
        - GET/network/greylist/ips
        - GET/network/greylist/peers
        - GET/network/self
        - GET/network/stat
        - GET/network/version
        - GET/network/versions
        - GET/protocols
        - GET/protocols.*
        - GET/protocols.*/environment
        - GET/version
        - POST/chains/.*/blocks/.*/helpers
        - POST/chains/.*/blocks/.*/script
        - POST/chains/.*/blocks/.*/context/contracts.*/big_map_get
        - POST/injection/operation
cache:
    disabled_routes:
        - GET/monitor/.*
        - GET/chains/.*/mempool
        - GET/chains/.*/blocks.*head
    enabled: true
    size_mb: 100
    ttl: 5
cors:
    enabled: true
    allow_headers:
    - '*'
    allow_origins:
    - '*'
    allow_methods:
    - GET
    - POST
    - OPTIONS
deny_ips:
    enabled: false
    values: []
deny_routes:
    enabled: true
    values:
        - GET/workers.*
        - GET/worker.*
        - GET/stats.*
        - GET/chains/.*/blocks/.*/helpers/baking_rights
        - GET/chains/.*/blocks/.*/helpers/endorsing_rights
        - GET/helpers/baking_rights
        - GET/helpers/endorsing_rights
        - GET/chains/.*/blocks/.*/context/contracts(/?)$
        - GET/chains/.*/blocks/.*/context/raw/bytes
        - POST/injection/block
        - POST/injection/protocol
dev_mode: false
gc:
    optimize_memory_store: true
    percent: 100
gzip:
    enabled: true
host: 0.0.0.0:8080
load_balancer:
    ttl: 600
logger:
    bunch_size: 1000
    pool_interval_seconds: 1
metrics:
    enabled: true
    host: 0.0.0.0:9000
    pprof: false
rate_limit:
    enabled: false
    max: 300
    minutes: 1
redis:
    enabled: false
    host: ""
mavryk_host:
    - 127.0.0.1:8732
mavryk_host_retry: ""
```

### Environment Variables

You can also configure or overwrite MvProxy with environment variables, using the same structure:

- `MVPROXY_DEV_MODE` is a flag to enable dev features like pretty logger.
- `MVPROXY_HOST` is the host of the proxy.
- `MVPROXY_MAVRYK_HOST` are the hosts of the mavryk nodes.
- `MVPROXY_MAVRYK_HOST_RETRY` is the host used when finding a 404 or 410. It's recommended use full or archive nodes.
- `MVPROXY_REDIS_HOST` is the host of the redis.
- `MVPROXY_REDIS_ENABLE` is a flag to enable redis.
- `MVPROXY_LOAD_BALANCER_TTL` is the time to live to keep using the same node by user IP.
- `MVPROXY_LOGGER_BUNCH_SIZE` is the bunch size of the logger.
- `MVPROXY_LOGGER_POOL_INTERVAL_SECONDS` is the pool interval of the logger.
- `MVPROXY_CACHE_ENABLED` is the flag to cache enable cache.
- `MVPROXY_CACHE_DISABLED_ROUTES` is the variable with the routes to cache.
- `MVPROXY_CACHE_SIZE_MB` is the size of the cache in megabytes.
- `MVPROXY_CACHE_TTL` is the time to live in seconds for cache.
- `MVPROXY_RATE_LIMIT_ENABLED` is a flag to enable rate limiting.
- `MVPROXY_RATE_LIMIT_MINUTES` is the minutes of the period of rate limiting. 
- `MVPROXY_RATE_LIMIT_MAX` is the max of requests permitted in a period.
- `MVPROXY_DENY_IPS_ENABLED` is a flag to block IP addresses.
- `MVPROXY_DENY_IPS_VALUES` is the IP Address that will be blocked on the proxy.
- `MVPROXY_DENY_ROUTES_ENABLED` is a flag to block the Mavryk node's routes. 
- `MVPROXY_DENY_ROUTES_VALUES` is the Mavryk nodes routes that will be blocked on the proxy.conf.
- `MVPROXY_ALLOW_ROUTES_ENABLED` is a flag to allow the Mavryk node's routes. 
- `MVPROXY_ALLOW_ROUTES_VALUES` is the Mavryk nodes routes that will be allowed on the proxy.conf.
- `MVPROXY_METRICS_ENABLED` is the flag to enable metrics.
- `MVPROXY_METRICS_PPROF` is the flag to enable pprof.
- `MVPROXY_METRICS_HOST` is the host of the prometheus metrics and pprof (if enabled).
- `MVPROXY_CORS_ENABLED` is the flag to enable cors.
- `MVPROXY_GZIP_ENABLED` is the flag to enable gzip.
- `MVPROXY_GC_OPTIMIZE_MEMORY_STORE` is a flag to optimize GC when it's using storage as memory allocations instead of redis.
- `MVPROXY_GC_PERCENT` is the percent of the garbage collector.

