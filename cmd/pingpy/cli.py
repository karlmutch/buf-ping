import argparse
import ssl

import grpc
import ping.v1.ping_pb2 as protobufs
import ping.v1.ping_pb2_grpc as grpc_interface


def open_secure_grpc_channel(cert_file, server_address):

    creds = grpc.ssl_channel_credentials(open(cert_file, 'rb').read())
    return grpc.secure_channel(target=server_address, credentials=creds)


def ping(stub):
    print("Executing ping")
    # Make a Ping RPC call with 'Hello, Server!' message
    response = stub.Ping(protobufs.PingRequest())
    print('Server responded:', response)  # Print server's response


def sum(stub):
    print("Executing sum")

    # Open a stream to the server, written using phind-codellama:34b-v2
    response_iterator = stub.Sum.future(
        iter([protobufs.SumRequest(addition=1)] * 6))

    # Wait for the response and get the sum value
    response = response_iterator.result()
    print("Received sum: {}".format(response.sum))


def generate(stub):
    print("Executing generate")


def count(stub):
    print("Executing count")


def hardfail(stub):
    print("Executing hardfail")


def main():
    parser = argparse.ArgumentParser(description='A Buf Ping client.')
    parser.add_argument('action', choices=['ping', 'sum', 'generate', 'count', 'hardfail'],
                        help='Choose an action: ping, sum, generate, count, hardfail')

    args = parser.parse_args()

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
        elif args.action == 'hardfail':
            hardfail(stub)


if __name__ == '__main__':
    main()
