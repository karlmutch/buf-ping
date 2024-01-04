# buf-ping

A server and client using CNCF connectrpc with Auth, [OpenTelemetry](https://opentelemetry.io), [gRPC Schemas BSR](https://buf.build).

The client and server are designed to show the four different types of gRPC message exchanges, unary, server streaming, client streaming, and bidirectional streaming.

## Building and build dependencies

Builds and tests (CI/CD) are done using dagger.io.  Dagger can be downloaded from the installation instructions at [Dagger Install](docs.dagger.io/cli).

Additional tooling that is useful for build activities include [jq](https://github.com/jqlang/jq), and access to a docker runtime such as [docker](https://docker.com), or our recommended platform for OSX [OrbStack](https://orbstack.dev).

When using an orbstack Linux VM and dagger together the docker command will need to be started using a script, for example:

```sh
# Replying to https://discord.com/channels/707636530424053791/1123687185837740053/1186907997532868618
#
# Hi Karl, we are working on a feature to customize how to run the engine container. In the meantime you could add a shell script called 
# `docker` in your shell’s search path:
#
cat > ~/.local/bin/docker <<‘EOF’
#!/bin/sh
mac docker $*
EOF
chmod +x ~/.local/bin/docker
```

OrbStack should also have its Docker configuration modified to support IPv6 by using the Settings menu item.

```sh
    "insecure-registries" : [ "registry.orb.local:5000" ]
```

The reference builds are done using the dagger.io engine, as shown in the following example.

```sh
docker run -d -p 5000:5000 --restart=always --name registry registry:2
dagger run go run ci/main.go
```

The dagger based build will cache between builds result in fresh builds taking 30 seconds per executable produced and 5 seconds once the cache is populated.

## Runtime Dependencies

### TLS Configuration

This example project is implemented as a production server and requires a TLS certificate to work properly.  The code is designed to emulate production code and not skip encryption etc and other steps that various styles of testing omit.

If you are testing then the following instructions can be used to create your own self signed certificate files.  For production cloud scenarios you should create a cloud provider signed certificate.  The following example creates two files, 'testing.key', and 'testing.crt' for use when running the server and client.

```sh
openssl req -newkey ec:<(openssl ecparam -name secp384r1) -nodes -keyout testing.key -x509 -days 180 -out testing.crt -subj '/C=US/ST=CA/L=Sonoma/O=Karl Mutch, INC/OU=Org' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1'
```

### OpenTelemetry Configuration

This project implements the OpenTelemetry framework for Observability.  It can be configured for use with the Honeycomb framework using the following configuration:

```sh
cat <<EOF >/tmp/otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        include_metadata: true
processors:
  batch:
    metadata_keys:
      - x-honeycomb-dataset
    metadata_cardinality_limit: 30
extensions:
  headers_setter:
    headers:
      - action: upsert
        key: x-honeycomb-dataset
        from_context: x-honeycomb-dataset
service:
  extensions:
    [ headers_setter ]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp]
      processors: [batch]
    metrics:
      receivers: [otlp]
      exporters: [otlp]
      processors: [batch]
    logs:
      receivers: [otlp]
      exporters: [otlp]
      processors: [batch]
exporters:
  otlp:
    endpoint: api.honeycomb.io:443
    headers:
      x-honeycomb-team: [YOUR_API_KEY]
    auth:
      authenticator: headers_setter
EOF
# If you are using OrbStack and invoking these commands from a Linux VM you will
# need to add the following command
#
# cp /tmp/otel-collector-config.yaml /mnt/mac/tmp/.
#
docker run --name otel_collector -p 4317:4317 \
    -v /tmp/otel-collector-config.yaml:/etc/otel-collector-config.yaml \
    otel/opentelemetry-collector-contrib:latest \
    --config=/etc/otel-collector-config.yaml
```

## Testing

For manual testing the grpcurl utility is used and can be obtained from <https://github.com/fullstorydev/grpcurl/releases>.

OpenTelemetry is sent to the HoneyComb OTel platform.  You can sign up for a free account at honeycomb.io and then access your account to create an API key. The environment can then be configured as follows:

```sh
export OTEL_SERVICE_NAME="ping-test-service"
export HONEYCOMB_API_KEY="your-api-key"
```

Additional information concerning OpenTelemetry can be found at [OTel General SDK configuration](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration).

```sh
go run ./cmd/pingsrv/.
```

```sh
$ grpcurl --insecure -H "authorization: Bearer $ACCESS_TOKEN" localhost:$PORT list
ping.v1.PingService
# Below is the secure variant when using self signed certificates
$ grpcurl --cacert testing.crt -H "authorization: Bearer $ACCESS_TOKEN" localhost:8080 describe
ping.v1.PingService is a service:
service PingService {
  rpc Count ( stream .ping.v1.CountRequest ) returns ( stream .ping.v1.CountResponse );
  rpc Generate ( .ping.v1.GenerateRequest ) returns ( stream .ping.v1.GenerateResponse );
  rpc HardFail ( .ping.v1.HardFailRequest ) returns ( .ping.v1.HardFailResponse );
  rpc Ping ( .ping.v1.PingRequest ) returns ( .ping.v1.PingResponse );
  rpc Sum ( stream .ping.v1.SumRequest ) returns ( .ping.v1.SumResponse );
}
```

```sh
# Zero values for the the sum at this are 0 which is the default value and will not be serialized on the wire
$ grpcurl --insecure -H "authorization: Bearer $ACCESS_TOKEN" -d '{}' localhost:8080 ping.v1.PingService/Ping
{
  "timestamp": "2023-12-19T19:24:03.432490224Z"
}
$ grpcurl --insecure -H "authorization: Bearer $ACCESS_TOKEN" -d '{"addition":"1"}{"addition":"1"}' localhost:8080 ping.v1.PingService/Count
{
  "sum": 1
}
{
  "sum": 2
}
$ grpcurl --insecure -H "authorization: Bearer $ACCESS_TOKEN" -d '{"addition":"1"}{"addition":"1"}' localhost:8080 ping.v1.PingService/Sum
{
  "sum": 4
}
$ grpcurl --insecure -H "authorization: Bearer $ACCESS_TOKEN" -d '{"addition":"1"}' localhost:8080 ping.v1.PingService/Generate
{
  "progress": 5
}
$ grpcurl --insecure -H "authorization: Bearer $ACCESS_TOKEN" -d '{"addition":"2"}' localhost:8080 ping.v1.PingService/Generate
{
  "progress": 6
}
{
  "progress": 7
}
```

```sh
grpcurl --insecure -H "authorization: Bearer $ACCESS_TOKEN" localhost:8080 describe grpc.health.v1.Health
grpc.health.v1.Health is a service:
service Health {
  rpc Check ( .grpc.health.v1.HealthCheckRequest ) returns ( .grpc.health.v1.HealthCheckResponse );
  rpc Watch ( .grpc.health.v1.HealthCheckRequest ) returns ( stream .grpc.health.v1.HealthCheckResponse );
}
$ grpcurl --insecure -H "authorization: Bearer $ACCESS_TOKEN" localhost:8080 grpc.health.v1.Health/Check
{
  "status": "SERVING"
}
```
