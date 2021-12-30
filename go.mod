module github.com/nuclio/nuclio

go 1.17

require (
	cloud.google.com/go/pubsub v1.10.2
	github.com/Azure/go-amqp v0.17.0
	github.com/Microsoft/ApplicationInsights-Go v0.4.2
	github.com/Shopify/sarama v1.23.1
	github.com/aws/aws-sdk-go v1.39.6
	github.com/coreos/go-semver v0.2.0
	github.com/disintegration/imaging v1.6.0
	github.com/docker/distribution v2.7.1+incompatible
	github.com/eclipse/paho.mqtt.golang v1.2.0
	github.com/fatih/structs v1.1.0
	github.com/gbrlsnchs/jwt/v3 v3.0.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-chi/chi/v5 v5.0.2
	github.com/go-chi/cors v1.0.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/gobuffalo/flect v0.2.2
	github.com/google/go-cmp v0.5.5
	github.com/hashicorp/go-uuid v1.0.1
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/icza/dyno v0.0.0-20180601094105-0c96289f9585
	github.com/imdario/mergo v0.3.12
	github.com/jarcoal/httpmock v1.0.4
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
	github.com/mholt/archiver/v3 v3.5.1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/nats-io/go-nats v1.7.2
	github.com/nuclio/errors v0.0.3
	github.com/nuclio/logger v0.0.1
	github.com/nuclio/logger-appinsights v0.0.1
	github.com/nuclio/nuclio-sdk-go v0.3.0
	github.com/nuclio/zap v0.0.6
	github.com/olekukonko/tablewriter v0.0.1
	github.com/prometheus/client_golang v1.1.0
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a
	github.com/robfig/cron v1.2.0
	github.com/rs/xid v1.2.1
	github.com/satori/go.uuid v1.2.0
	github.com/sendgridlabs/go-kinesis v0.0.0-20190306160747-8de9069567f6
	github.com/spf13/cobra v0.0.5
	github.com/streadway/amqp v0.0.0-20190815230801-eade30b20f1d
	github.com/stretchr/testify v1.7.0
	github.com/tsenart/vegeta/v12 v12.8.4
	github.com/v3io/scaler v0.5.1
	github.com/v3io/v3io-go v0.2.21
	github.com/v3io/v3io-go-http v0.0.1
	github.com/v3io/version-go v0.0.2
	github.com/valyala/fasthttp v1.30.0
	github.com/vmihailenco/msgpack/v4 v4.3.12
	golang.org/x/oauth2 v0.0.0-20210402161424-2e8d93401602
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/api v0.43.0
	google.golang.org/grpc v1.36.1
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.8
	k8s.io/apimachinery v0.21.8
	k8s.io/client-go v0.21.8
	k8s.io/code-generator v0.21.8
	k8s.io/metrics v0.21.8
)

require (
	cloud.google.com/go v0.81.0 // indirect
	code.cloudfoundry.org/clock v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.4.16 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20210428141323-04723f9f07d7 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/andybalholm/brotli v1.0.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cpuguy83/go-md2man v1.0.10 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dsnet/compress v0.0.2-0.20210315054119-f66993602bf5 // indirect
	github.com/eapache/go-resiliency v1.1.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/emicklei/go-restful v2.9.5+incompatible // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.3 // indirect
	github.com/go-openapi/jsonreference v0.19.3 // indirect
	github.com/go-openapi/spec v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/googleapis/gax-go/v2 v2.0.5 // indirect
	github.com/googleapis/gnostic v0.4.1 // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/influxdata/tdigest v0.0.0-20180711151920-a7d76c6f093a // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jcmturner/gofork v0.0.0-20190328161633-dc7c13fece03 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.10 // indirect
	github.com/jstemmer/go-junit-report v0.9.1 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20201106050909-4977a11b4351 // indirect
	github.com/klauspost/compress v1.13.4 // indirect
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/magefile/mage v1.11.0 // indirect
	github.com/mailru/easyjson v0.7.0 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/nats-io/gnatsd v1.4.1 // indirect
	github.com/nats-io/nkeys v0.2.0 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pavius/zap v1.4.2-0.20180228181622-8d52692529b8 // indirect
	github.com/pierrec/lz4 v2.2.6+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4 // indirect
	github.com/prometheus/common v0.6.0 // indirect
	github.com/prometheus/procfs v0.0.3 // indirect
	github.com/russross/blackfriday v1.5.2 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/ulikunitz/xz v0.5.9 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/vmihailenco/tagparser v0.1.1 // indirect
	github.com/xanzy/ssh-agent v0.3.0 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/image v0.0.0-20190802002840-cff245a6509b // indirect
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/mod v0.4.1 // indirect
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d // indirect
	golang.org/x/text v0.3.6 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	golang.org/x/tools v0.1.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210406143921-e86de6bf7a46 // indirect
	google.golang.org/protobuf v1.26.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/jcmturner/aescts.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/dnsutils.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/gokrb5.v7 v7.2.3 // indirect
	gopkg.in/jcmturner/rpc.v1 v1.1.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
	k8s.io/gengo v0.0.0-20201214224949-b6c5ce23f027 // indirect
	k8s.io/klog/v2 v2.9.0 // indirect
	k8s.io/kube-openapi v0.0.0-20211110012726-3cc51fd1e909 // indirect
	k8s.io/utils v0.0.0-20210521133846-da695404a2bc // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
	zombiezen.com/go/capnproto2 v2.17.0+incompatible // indirect
)

replace github.com/Shopify/sarama => github.com/iguazio/sarama v1.25.1-0.20201117150928-15517d41c014
