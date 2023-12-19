# buf-ping

A server and client using CNCF connectrpc with Auth, OTel, BSR, http interceptor examples also included.

Builds will be done using dagger.io.

## Runtime Dependencies

This example project is implemented as a production server and requires a TLS certificate to work properly.  The code is designed to emulate production code and not skip encryption etc and other steps that variopus styles of testing omit.

If you are testing then the following instructions can be used to create your own self signed certificate files.  For production cloud scenarios you should create a cloud provider signed certificate.  The following example creates two files, 'testing.key', and 'testing.crt' for use when running the server and client.

```sh
openssl req -newkey ec:<(openssl ecparam -name secp384r1) -nodes -keyout testing.key -x509 -days 180 -out testing.crt -subj '/C=US/ST=CA/L=Sonoma/O=Karl Mutch, INC/OU=Org' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1'
```
