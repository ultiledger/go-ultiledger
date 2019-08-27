# Go Ultiledger
Official Go implementation of the Ultiledger network.

# How to build

## Building from source
The primary command binary of Ultiledger is `ult`. You need to have Go (version 1.12 or later) installed for building `ult`. Then run

```shell
cd cmd/ult && go build
```

That's it! Now you can use the binary `ult` to start creating your own Ultiledger nodes.

## Using Docker
If you have Docker properly installed, you can just run

```shell
docker run ult:latest help
```

to get start.

# Executables

There are other helpful command binaries alongwith the main `ult`.

|    Command    | Description |
| :-----------: | ----------- |
|    `ult`    | The main command binary for bootstrapping a Ultiledger node which can participate the consensus or just watch the consensus messages. |
|   `ultcli`  | The utility command binary for generating IDs for nodes and accounts. |
|   `ulttest` | It is used for running defined test cases in test network. |

## Running `ult`

For bootstrapping a new node, we shall run the following command:

```shell
./ult start --newnode --config /path/to/your/config.yaml
```

The command will:
  * Initialize the database with the specified name and location in the config file.
  * Create the master account and log the account information in the console.
  * Close the genesis ledger.

If the node is crashed for some reasons and we want to recover the node, we should run:

```shell
./ult start --config /path/to/your/config.yaml
```

The command will load the lastest checkpoint of the ledger and try to move forward with the current states.

# Client

The `client` package contains the necessary libraries to interact with the Ultiledger network. 
