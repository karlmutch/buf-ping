# buf-ping

A server and client using CNCF connectrpc with Auth, OTel, BSR, http interceptor examples also included.

Builds will be done using dagger.io.

## Runtime Dependencies

This example project is implemented as a production server and requires a TLS certificate to work properly.  The code is designed to emulate production code and not skip encryption etc and other steps that variopus styles of testing omit.

If you are testing then the following instructions can be used to create your own self signed certificate files.  For production cloud scenarios you should create a cloud provider signed certificate.  The following example creates two files, 'testing.key', and 'testing.crt' for use when running the server and client.

```sh
openssl req -newkey ec:<(openssl ecparam -name secp384r1) -nodes -keyout testing.key -x509 -days 180 -out testing.crt -subj '/C=US/ST=CA/L=Sonoma/O=Karl Mutch, INC/OU=Org' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1'
```

## Testing

For manual testing the grpcurl utility is used and can be obtained from https://github.com/fullstorydev/grpcurl/releases.

OpenTelemetry is sent to the HoneyComb OTel platform.  Ypu can sign up for a free account at honeycomb.io and then access your account to create an API key. The environment can then be configured as follows:

```sh
export OTEL_SERVICE_NAME="ping-test-service"
export HONEYCOMB_API_KEY="your-api-key"

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
