import argparse
import ssl

import grpc
import ping.v1.ping_pb2 as protobufs
import ping.v1.ping_pb2_grpc as grpc_interface


def open_secure_grpc_channel(cert_file, server_address):

    creds = grpc.ssl_channel_credentials(open(cert_file, 'rb').read())
    return grpc.secure_channel(target=server_address, credentials=creds)


def ping():
    print("Executing ping")
    # Open a gRPC channel to localhost on port 50051
    with open_secure_grpc_channel('../../testing.crt', 'localhost:8080') as channel:
        # Create a gRPC stub from the channel
        stub = grpc_interface.PingServiceStub(channel)
        # Make a Ping RPC call with 'Hello, Server!' message
        response = stub.Ping(protobufs.PingRequest())
        print('Server responded:', response)  # Print server's response


def sum():
    print("Executing sum")


def generate():
    print("Executing generate")


def count():
    print("Executing count")


def hardfail():
    print("Executing hardfail")


def main():
    parser = argparse.ArgumentParser(description='A Buf Ping client.')
    parser.add_argument('action', choices=['ping', 'sum', 'generate', 'count', 'hardfail'],
                        help='Choose an action: ping, sum, generate, count, hardfail')

    args = parser.parse_args()

    if args.action == 'ping':
        ping()
    elif args.action == 'sum':
        sum()
    elif args.action == 'generate':
        generate()
    elif args.action == 'hardfail':
        hardfail()


if __name__ == '__main__':
    main()
