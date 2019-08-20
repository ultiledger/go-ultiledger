# Go Ultiledger
Official Go implementation of the Ultiledger network.

# How to build

## Building from source
The primary command binary of Ultiledger is `ult`. You need to have Go (version 1.12 or later) installed for building `ult`. Then run

```shell
cd cmd/ult && go build
```

That's it! Now you can use the binary `cmd/ult/ult` to start creating your own Ultiledger nodes.

## Using Docker
If you have Docker properly installed, you can just run

```shell
docker run ult:latest help
```

to get start.
