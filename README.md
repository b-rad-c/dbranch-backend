# dBranch Backend

The backend code for the dbranch node. It is a work in progress and will have more features added over time. Right now its functionality is limited to listening to an IPFS pubsub channel for new articles and then copying them to the local ipfs node's curated directory. This is slightly different than pinning files because the publisher does not have to pin the file for a curator to host it. The curator can configure a list of peers to curate from to prevent all content from being hosted.

### setup

Your IPFS node needs to be started with the `--enable-pubsub-experiment` flag, [see here for more](ipns://docs.ipfs.io/reference/cli/#ipfs-pubsub)

### cli
This project is still in alpha phase, run the followin command to see the current cli options

    go run main.go help


### configuration
Configuration is done through a json file. The path can be supplied with cli flags `-c` or `--config` or with env variable `DBRANCH_CURATOR_CONFIG`. If neither or found the default path will be used `~/.dbranch/curator.json`. If the path doesn't exist the following default file will be written to it and used.
**default config**
    
    {
        "allow_empty_peer_list": false,
        "curated_dir": "/dBranch/curated",
        "ipfs_host": "localhost:5001",
        "log_path": "-",
        "allowed_peers": [
        ],
        "wire_channel": "dbranch-wire"
    }


`ipfs_host` - the address of the local ipfs node, default: `localhost:5001`

`wire_channel` - the IPFS [pubsub topic](ipns://docs.ipfs.io/reference/cli/#ipfs-pubsub) to listen for new articles on, default: `dbranch-wire`

`curated_dir` - the IPFS files directory to copy curated articles into, default: `/dBranch/curated`

`allowed_peers` - a list of ipfs peer ids to limit whose articles will be curated, see section below for more details.

`allow_empty_peer_list` - if `true` the program will exit if the allow list file does not exist or the list is empty (which implies all articles will be curated), default: `false`

`log_path` - the file to log to or `-` for stdout, default: `-`

### allowed peers list

The peer allow list contains IPFS peer ids that will be automatically curated to the local IPFS node when a new article is received on the `wire_channel` pubsub. If the list is empty all articles will be automatically curated, by default this behaviour is not allowed, see `allow_empty_peer_list` in the configuration section to override.

To get the peer id for an IPFS node run the [ipfs id](ipns://docs.ipfs.io/reference/cli/#ipfs-id) command:

    $ ipfs id
    {
            "ID": "<THIS IS YOUR PEER ID>",
            "PublicKey": "...",
            "Addresses": [
                    "...",
                    "..."
            ],
            "AgentVersion": "...",
            "ProtocolVersion": "...",
            "Protocols": [
                    "..."
            ]
    }
