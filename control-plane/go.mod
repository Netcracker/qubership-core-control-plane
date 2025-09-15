module github.com/netcracker/qubership-core-control-plane/control-plane/v2

go 1.24.2

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/docker/docker v27.5.1+incompatible
	github.com/docker/go-connections v0.5.0
	github.com/envoyproxy/go-control-plane v0.13.4
	github.com/envoyproxy/go-control-plane/envoy v1.32.4
	github.com/fasthttp/websocket v1.5.12
	github.com/ghodss/yaml v1.0.0
	github.com/go-errors/errors v1.5.1
	github.com/gofiber/fiber/v2 v2.52.9
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.4
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/hashicorp/go-memdb v1.3.5
	github.com/hashicorp/go-uuid v1.0.3
	github.com/hashicorp/memberlist v0.5.3
	github.com/hashicorp/serf v0.10.2
	github.com/jackc/pgconn v1.14.3
	github.com/jackc/pgx/v5 v5.7.5
	github.com/jellydator/ttlcache/v3 v3.3.0
	github.com/lib/pq v1.10.9
	github.com/mitchellh/mapstructure v1.5.0
	github.com/mustafaturan/bus/v3 v3.0.3
	github.com/mustafaturan/monoton/v3 v3.0.0
	github.com/netcracker/qubership-core-lib-go-actuator-common/v2 v2.0.0-20250505145427-766e76edee70
	github.com/netcracker/qubership-core-lib-go-dbaas-base-client/v3 v3.0.0-20250505145933-fe8e90a2dbd8
	github.com/netcracker/qubership-core-lib-go-dbaas-postgres-client/v4 v4.0.0-20250505150011-8af5ff8da9f2
	github.com/netcracker/qubership-core-lib-go-error-handling/v3 v3.0.0-20250403062747-4cb66dc1cc62
	github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2 v2.0.0-20250505150321-09ac8e0e3ac2
	github.com/netcracker/qubership-core-lib-go-rest-utils/v2 v2.0.0-20250507111222-22b01ae759f8
	github.com/netcracker/qubership-core-lib-go-stomp-websocket/v3 v3.0.0-20250410104403-5fc53287436a
	github.com/netcracker/qubership-core-lib-go/v3 v3.0.0-20250508122204-5cb3c4abfd2b
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.22.0
	github.com/puzpuzpuz/xsync/v3 v3.5.1
	github.com/stretchr/testify v1.10.0
	github.com/swaggo/swag v1.16.6
	github.com/tidwall/gjson v1.18.0
	github.com/uptrace/bun v1.2.15
	github.com/uptrace/bun/dialect/pgdialect v1.2.15
	github.com/uptrace/bun/driver/pgdriver v1.2.9
	github.com/urfave/cli/v2 v2.27.7
	github.com/valyala/fasthttp v1.61.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250505200425-f936aa4a68b2
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.9
	sigs.k8s.io/yaml v1.4.0
)

require (
	cel.dev/expr v0.19.0 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/avast/retry-go/v4 v4.6.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20240905190251-b4127c9b8d78 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/envoyproxy/go-control-plane/ratelimit v0.1.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/spec v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gofiber/adaptor/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/hashicorp/consul/api v1.31.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.6 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.3 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/miekg/dns v1.1.56 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.63.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/savsgio/gotils v0.0.0-20240704082632-aef3928b8a38 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/vibrantbyte/go-antpath v1.1.1 // indirect
	github.com/viney-shih/go-lock v1.1.2 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.51.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.35.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/zipkin v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp v0.0.0-20230817173708-d852ddb80c63 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241202173237-19429a94021a // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.3.0 // indirect
	mellium.im/sasl v0.3.2 // indirect
)
