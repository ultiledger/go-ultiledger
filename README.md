## Go Ultiledger
Official Go implementation of the Ultiledger network.

[![API Reference](
https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667
)](https://godoc.org/github.com/ultiledger/go-ultiledger)

## How to build

### Building from source
The primary command binary of Ultiledger is `ult`. You need to have Go (version 1.12 or later) installed for building `ult`. Then run

```shell
cd cmd/ult && go build
```

That's it! Now you can use the binary `ult` to start creating your own Ultiledger nodes.

### Using Docker
If you have Docker properly installed, you can just run

```shell
docker run -it ultiledger/ult:latest-test start help
```

to get start.

## Executables

There are other helpful command binaries besides the main `ult`:

|    Command    | Description |
| :-----------: | ----------- |
|    `ult`    | The main command binary for bootstrapping a Ultiledger node which can participate the consensus or just watch the consensus messages. |
|   `ultcli`  | The utility command binary for generating IDs for nodes and accounts. |
|   `ulttest` | It is used for running defined test cases in test network. |

### Running `ult`

For bootstrapping a new node, we shall run the following command:

```shell
./ult start  --config /path/to/your/config.yaml --newnode
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

### Running `ultcli`

The `ultcli` can be used to generate a random node id or a random account id.

* Generate a random account id.

```shell
// Command
./ultcli genaccountid

// Output
AccountID: XGXokiYjAQ52ud4ZzefLMnPHjLhdCFt5Z5ceHoaBpZcm, Seed: d9pxE1ugN21tVheB31w5vDcrv71eK7dExYqrqw15jePn
```

* Generate a random node id.

```shell
// Command
./ultcli gennodeid

// Output
NodeID: 2kPPBJuKvwSpBfLgSAxs3Rd5FSmRMonMPT2cT2eRLDv7f, Seed: oWCA5t5dazfWjtfLP254gLznV8mS54wa514F337VQhNK
```

## Config

The `ult` command binary relies on a config file to work. The full configuration parameters can be found in `config.example`.
The most import parameter to set is in the `quorum` section. Ultiledger allows the node to choose their quorum in a decentralized way.
That is the node has to decide their quorum by itself instead of being provided by the network. 

For example:

```yaml
quorum:
  threshold: 0.51
  validators:
    - "2WBKhr1dCnpAp1iMUZ8WU43y4dVqc5BbexUxaHLRS4DVq"
    - "2cQbFofKksdpfC87HpimsKtwWbyAUwHn54bHmMeNXiv7o"
    - "2jeyThunzi1eyEcJsnHDHkLfvXjp9kdzcDAXTUUVzoinE"
```

The nodes in the quorum should have the role of `validator` as a `watcher` node will not participate any voting processes. The validators are identified with their `node_id`s. The node decides that there are 3 validators in its quorum and the threshold for the node to agree on any decision is 0.51. We take the ceiling of `0.51 * 3`, which is 2,  as the integer threshold to decide whether the node should accept any consensus decision from the quorum.

## Client

Clients interact with the Ultiledger network by submitting well-formed transactions. Only after being confirmed by the network through consensus, transactions could be regarded as valid.

### Example

The following snippet shows the core operations needed to submit a point-to-point payment transaction to the network.

```go
  // The account id and seed of the source account.
  srcAccountID, srcSeed := ......
  // The destination account to receive the payment.
  dstAccountID := ......
  
  // Query the source account to find out the current sequence number.
  srcAccount, err := clt.GetAccount(srcAccountID)
  if err != nil {
    return err
  }

  // Mutators are operators to maniputate the transaction.
  var mutators []build.TxMutator
  mutators = append(mutators, &build.AccountID{AccountID: srcAccountID})
  mutators = append(mutators, &build.Payment{
    AccountID: dstAccountID,
    Amount:    int64(10000000000), // Pay 1 ULT
    Asset:     &build.Asset{AssetType: build.NATIVE},
  })
  mutators = append(mutators, &build.SeqNum{SeqNum: srcAccount.SeqNum + 1})

  // Apply the mutators to the transaction.
  tx := build.NewTx()
  err = tx.Add(mutators...)
  if err != nil {
    return fmt.Errorf("build tx failed: %v", err)
  }

  // Sign the transaction with the source account seed.
  payload, signature, err := tx.Sign(srcSeed)
  if err != nil {
    return fmt.Errorf("sign payment tx failed: %v", err)
  }

  // Compute the hash key of the transaction.
  txKey, err := tx.GetTxKey()
  if err != nil {
    return fmt.Errorf("get tx key failed: %v", err)
  }

  // Submit the tx.
  err = cli.SubmitTx(txKey, signature, payload)
  if err != nil {
    return fmt.Errorf("submit tx failed: %v", err)
  }
  // If we arrive in here, it means the transaction has passed
  // necessary admission control checks but not applied yet. We
  // can use the computed `txKey` to query the status of the
  // transaction later on.
```

See the [test](test) folder for the full code and other examples.

## Testnet

In Ultiledger network, there are two different roles of a node: `watcher` and `validator`. As a `watcher` node, it can only listen for consensus messages from peers without the ability to participate the voting process of the consensus protocol. If you want the node to participate the consensus process, you need to set the role of the node to `validator`. See [config.example](config.example) for details.

We provide a config template for the testnet which contains the trusted existing validators of the testnet. Users need to generate a pair of keys (node id and node seed) for the node using `ultcli`, then substitude the keys in the corresponding fields of [config.testnet](config.testnet).

As an example of bootstrapping a new node, we run:

```shell
docker run -it --network host \
  -v /root/ultiledger:/config \
  -v /root/ultiledger:/db \
  -v /root/ultiledger:/log \
ultiledger/ult:latest-testnet start --config /config/config.testnet.yaml --newnode
```

we assume the local path `/root/ultiledger` contains the mounted config file, the database file and the log file.

For recovering an existing node, the `--newnode` flag is not needed.

## Contribution

Everyone are welcomed to contribute to this project, even tiny corrections for typos will be beneficial to the project. As this project is very fresh and under active developments, we wish to provide some guidelines for the protential contributors so as to minimize undesired pull requests:

 * Use [gofmt](https://golang.org/cmd/gofmt/) to format your codes.
 * Follow the official Go [commentary](https://golang.org/doc/effective_go.html#commentary) guidelines.
 * Pull requests need to be based on and opened against the `master` branch.
 * Commit messages should be capitalized in the first word and be ended with a period.
