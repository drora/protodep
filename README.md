protodep
=======

![logo](./logo/web.png)


![GitHub Actions](https://github.com/stormcat24/protodep/actions/workflows/go.yml/badge.svg)
[![Language](https://img.shields.io/badge/language-go-brightgreen.svg?style=flat)](https://golang.org/)
[![issues](https://img.shields.io/github/issues/stormcat24/protodep.svg?style=flat)](https://github.com/stormcat24/protodep/issues?state=open)
[![License: MIT](https://img.shields.io/badge/license-MIT-orange.svg)](LICENSE)
[![GoDoc](https://godoc.org/github.com/stormcat24/protodep?status.png)](https://godoc.org/github.com/stormcat24/protodep)

A dependency vendoring manager for Protocol Buffers IDL file (.proto).

Forked from original in order to implement smart proto patch capability:  
if configured, to prevent collisions, this tool will automatically change the proto package according to local dir.
Also, it will add a message option to your choice with the original path (package+message) so this info will not be lost.  
Example:
```toml
proto_outdir = "./path/to/proto/upstream"
patch_package_with_message_annotation = "api.submessage_of"

[[dependencies]]
target = "github.com/grpc-ecosystem/grpc-gateway/examples/examplepb"
revision = "v1.2.2"
path = "grpc-gateway/examplepb"
```


## Motivation

In building Microservices architecture, gRPC with Protocol Buffers is effective. When using gRPC, your application will depend on many remote services.
If you manage proto files in a git repository, what will you do? Most remote services are managed by git and they will be versioned. We need to control which dependency service version that application uses.


## Install

### from binary

Support as follows:

* `protodep_darwin_amd64.tar.gz`
* `protodep_linux_386.tar.gz`
* `protodep_linux_amd64.tar.gz`
* `protodep_linux_arm.tar.gz`
* `protodep_linux_arm64.tar.gz`

```bash
$ wget https://github.com/drora/protodep/releases/download/0.9.4/protodep_darwin_amd64.tar.gz
$ tar -xf protodep_darwin_amd64.tar.gz
$ mv protodep /usr/local/bin/
```

## Usage

### protodep.toml

Proto dependency management is defined in `protodep.toml`.

```toml
proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/stormcat24/protodep/protobuf"
  branch = "master"

[[dependencies]]
  target = "github.com/grpc-ecosystem/grpc-gateway/examples/examplepb"
  revision = "v1.2.2"
  path = "grpc-gateway/examplepb"

[[dependencies]]
  target = "github.com/kubernetes/helm/_proto/hapi"
  branch = "master"
  path = "helm/hapi"
  ignores = ["./release", "./rudder", "./services", "./version"]
```

### protodep up

In same directory, execute this command.

```bash
$ protodep up
```

If succeeded, `protodep.lock` is generated.

### protodep up -f (force update)

Even if protodep.lock exists, you can force update dependenies.

```bash
$ protodep up -f
```

### [Attention] Changes from 0.1.0

From protodep 0.1.0 supports ssh-agent, and this is the default.
In other words, in order to operate protodep without options as before, it is necessary to set with ssh-add.

As the follows:

```bash
$ ssh-add ~/.ssh/id_rsa
$ protodep up
```

### Getting to private repo dependencies via HTTPS

If you want to get it via HTTPS, do as follows.

```bash
$ protodep up --use-https
```

And also, if Basic authentication is required, do as follows.
If you have 2FA enabled, specify the Personal Access Token as the password. 

```bash
$ protodep up --use-https \
    --basic-auth-username=your-github-username \
    --basic-auth-password=your-github-password
```

### [Attention] ssh VS https caveat

If you used to access a dependency via ssh, and now you want to access via https,
you need to invalidate local cache first or else you will get an error:
```shell
fetch repository is failed: ssh: handshake failed: knownhosts: key is unknown
```
to overcome this situation just run with a cleanup flag:
```bash
$ protodep up --cleanup --use-https \
    --basic-auth-username=your-github-username \
    --basic-auth-password=your-github-password
```

License
===
See [LICENSE](LICENSE).

Copyright © stromcat24. All Rights Reserved.
