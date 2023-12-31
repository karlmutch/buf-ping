# Python gRPC client for ping-buf

This project contains a CLI interface for illustraiting the code needed to connect to
the buf-ping server and perform all of the operations supported by the server.

To learn more about python and gRPC please refer to [gRPC and Python](https://grpc.io/docs/languages/python/).

## Build

### Python Builder

This project makes use of the poetry python packing tooling, see more information for the installtion and use of poetry at [Python Poetry](https://python-poetry.org/).

To start development use the following command:

```sh
cd ./cmd/pingpy
sudo apt-get install build-essential zlib1g-dev libffi-dev libssl-dev libbz2-dev libreadline-dev libsqlite3-dev liblzma-dev
pyenv install 3.10
# The next step will create a .python-version file in the cmd/pingpy directory
pyenv local 3.10
python -m venv .venv
source .venv/bin/activate
chmod +x .venv/bin/activate
poetry install

```

The original project had package sources and implementations added using the following commands:

```sh
poetry add opentelemetry-sdk
poetry add opentelemetry-api
poetry add grpcio
poetry add grpcio-tools
poetry add opentelemetry-instrumentation-grpc
poetry add opentelemetry-instrumentation-logging
poetry add opentelemetry-exporter-otlp
poetry source add buf.build https://buf.build/gen/python'
```

The packages inside the github version of this project already contains these dependencies. For more information please see, [Poetry Package Sources](https://python-poetry.org/docs/repositories/#supported-package-sources).

The Buf.Buf generated python code was added from the public account karlmutch using:

```sh
poetry add karlmutch-bufping-protocolbuffers-pyi
poetry add karlmutch-bufping-grpc-python
poetry add karlmutch-bufping-protocolbuffers-python
```

If you are using this project with VSCode, please refer to the following project, [Poetry and VSCode](https://www.pythoncheatsheet.org/blog/python-projects-with-poetry-and-vscode-part-1#creating-a-virtual-environment).

```sh
# Information about host naming can be found at https://docs.orbstack.dev/docker/network#domain-names
export OTEL_EXPORTER_OTLP_ENDPOINT=http://otel_collector.orb.local:4317/
# https://www.honeycomb.io/blog/simplify-opentelemetry-pipelines-headers-setter
export OTEL_EXPORTER_OTLP_HEADERS="x-honeycomb-team=[MY_API_KEY]"
```


### OpenTelemetry Configuration

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
