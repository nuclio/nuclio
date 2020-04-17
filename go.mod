module github.com/nuclio/nuclio

go 1.14

require (
	cloud.google.com/go v0.55.0 // indirect
	cloud.google.com/go/pubsub v1.2.0
	code.cloudfoundry.org/clock v1.0.0 // indirect
	github.com/Azure/azure-sdk-for-go v36.0.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.9.2 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.8.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/Microsoft/ApplicationInsights-Go v0.4.2
	github.com/Shopify/sarama v1.23.1
	github.com/aws/aws-sdk-go v1.30.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/disintegration/imaging v1.6.0
	github.com/eclipse/paho.mqtt.golang v1.2.0
	github.com/fatih/structs v1.1.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/go-chi/cors v1.0.0
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/hashicorp/go-uuid v1.0.1
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/howeyc/gopass v0.0.0-20170109162249-bf9dde6d0d2c // indirect
	github.com/icza/dyno v0.0.0-20180601094105-0c96289f9585
	github.com/imdario/mergo v0.3.7
	github.com/jarcoal/httpmock v1.0.4
	github.com/klauspost/compress v1.10.3 // indirect
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/mholt/archiver/v3 v3.3.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/nats-io/gnatsd v1.4.1 // indirect
	github.com/nats-io/go-nats v1.7.2
	github.com/nats-io/nkeys v0.1.0 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/nuclio/errors v0.0.2
	github.com/nuclio/logger v0.0.1
	github.com/nuclio/logger-appinsights v0.0.1
	github.com/nuclio/nuclio-sdk-go v0.0.1
	github.com/nuclio/zap v0.0.2
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/olekukonko/tablewriter v0.0.1
	github.com/onsi/ginkgo v1.10.0 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/pierrec/lz4 v2.4.1+incompatible // indirect
	github.com/prometheus/client_golang v1.1.0
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a
	github.com/robfig/cron v1.2.0
	github.com/rs/xid v1.2.1
	github.com/satori/go.uuid v1.2.0
	github.com/sendgridlabs/go-kinesis v0.0.0-20190306160747-8de9069567f6
	github.com/spf13/cobra v0.0.5
	github.com/streadway/amqp v0.0.0-20190815230801-eade30b20f1d
	github.com/streadway/quantile v0.0.0-20150917103942-b0c588724d25 // indirect
	github.com/stretchr/testify v1.5.1
	github.com/tedsuo/ifrit v0.0.0-20180802180643-bea94bb476cc // indirect
	github.com/tsenart/vegeta v6.3.0+incompatible
	github.com/ulikunitz/xz v0.5.7 // indirect
	github.com/v3io/scaler-types v1.6.0
	github.com/v3io/v3io-go v0.1.4
	github.com/v3io/v3io-go-http v0.0.1
	github.com/valyala/fasthttp v1.9.0
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	go.uber.org/atomic v1.4.0 // indirect
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	golang.org/x/tools v0.0.0-20200331025713-a30bf2db82d4 // indirect
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.0.0-20180713172427-0f11257a8a25
	k8s.io/apimachinery v0.0.0-20180619225948-e386b2658ed2
	k8s.io/client-go v7.0.0+incompatible
	pack.ag/amqp v0.12.5
)

replace github.com/Shopify/sarama => github.com/iguazio/sarama v1.25.1-0.20200331135945-d92101249c96
