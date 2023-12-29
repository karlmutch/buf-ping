# Python gRPC client for ping-buf

This project contains a CLI interface for illustraiting the code needed to connect to
the buf-ping server and perform all of the operations supported by the server.

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
poetry add grpcio
poetry add grpcio-tools
poetry source add buf.build https://buf.build/gen/python'
```

More information please see, [Poetry Package Sources](https://python-poetry.org/docs/repositories/#supported-package-sources).

The Buf.Buf generated python code was added from the public account karlmutch using:

```sh
poetry add karlmutch-bufping-protocolbuffers-pyi
poetry add karlmutch-bufping-grpc-python
poetry add karlmutch-bufping-protocolbuffers-python
```

If you are using this project with VSCode, please refer to the following project, [Poetry and VSCode](https://www.pythoncheatsheet.org/blog/python-projects-with-poetry-and-vscode-part-1#creating-a-virtual-environment).
