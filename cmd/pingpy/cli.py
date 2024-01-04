import argparse
import inspect
import logging

import grpc
import ping.v1.ping_pb2 as protobufs
import ping.v1.ping_pb2_grpc as grpc_interface
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import \
    OTLPSpanExporter
from opentelemetry.instrumentation.grpc import GrpcInstrumentorClient
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

trace.set_tracer_provider(TracerProvider())
tracer = trace.get_tracer_provider().get_tracer(__name__)
trace.get_tracer_provider().add_span_processor(
    BatchSpanProcessor(OTLPSpanExporter(insecure=True))
)


def open_secure_grpc_channel(cert_file, server_address):

    creds = grpc.ssl_channel_credentials(open(cert_file, 'rb').read())
    return grpc.secure_channel(target=server_address, credentials=creds)


def ping(stub):
    with tracer.start_as_current_span(inspect.currentframe().f_code.co_name):
        # Make a Ping RPC call with 'Hello, Server!' message
        response = stub.Ping(protobufs.PingRequest())
        print('Server responded:', response)  # Print server's response


def sum(stub):
    with tracer.start_as_current_span(inspect.currentframe().f_code.co_name):
        # Open a stream to the server, written using phind-codellama:34b-v2
        response_iterator = stub.Sum.future(
            iter([protobufs.SumRequest(addition=1)] * 6))

        # Wait for the response and get the sum value
        response = response_iterator.result()
        print("Received sum: {}".format(response.sum))


def generate(stub):
    with tracer.start_as_current_span(inspect.currentframe().f_code.co_name):
        response_iterator = stub.Generate(
            protobufs.GenerateRequest(addition=6))

        for response in response_iterator:
            print("Received incremental sum: {}".format(response.progress))


def generate_stream():
    with tracer.start_as_current_span(inspect.currentframe().f_code.co_name):
        for msg in iter([protobufs.CountRequest(addition=1)] * 6):
            yield msg


def count(stub):
    with tracer.start_as_current_span(inspect.currentframe().f_code.co_name):
        for response in stub.Count(generate_stream()):
            print("Received incremental sum: {}".format(response.sum))


def hardfail(stub):
    tracer.start_as_current_span(
        inspect.currentframe().f_code.co_name)


def main():
    parser = argparse.ArgumentParser(description='A Buf Ping client.')
    parser.add_argument('action', choices=['ping', 'sum', 'generate', 'count', 'hardfail'],
                        help='Choose an action: ping, sum, generate, count, hardfail')

    args = parser.parse_args()

    grpc_client_instrumentor = GrpcInstrumentorClient()
    grpc_client_instrumentor.instrument()

    hostPort = 'localhost:8080'
    with open_secure_grpc_channel('../../testing.crt', hostPort) as channel:
        # Create a gRPC stub from the channel
        stub = grpc_interface.PingServiceStub(channel)

        if args.action == 'ping':
            ping(stub)
        elif args.action == 'sum':
            sum(stub)
        elif args.action == 'generate':
            generate(stub)
        elif args.action == 'count':
            count(stub)
        elif args.action == 'hardfail':
            hardfail(stub)


if __name__ == '__main__':
    logging.basicConfig()

    main()
