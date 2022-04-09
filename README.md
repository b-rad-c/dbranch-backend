# dBranch Backend

The backend code for the dbranch node. It is a work in progress and will have more features added over time. Right now its functionality is limited to listening to an IPFS pubsub channel for new articles and then copying them to the local ipfs node's curated directory. This is slightly different than pinning files because the publisher does not have to pin the file for a curator to host it. The curator can configure a list of peers to curate from to prevent all content from being hosted.

### setup

Your IPFS node needs to be started with the `--enable-pubsub-experiment` flag, [see here for more](ipns://docs.ipfs.io/reference/cli/#ipfs-pubsub)

### configuration
Configuration is done through the following environment variables:

`IPFS_HOST` - the address of the local ipfs node, default: `localhost:5001`

`DBRANCH_WIRE_CHANNEL` - the IPFS [pubsub topic](ipns://docs.ipfs.io/reference/cli/#ipfs-pubsub) to listen for new articles on, default: `dbranch-wire`

`DBRANCH_CURATED_DIRECTORY` - the IPFS files directory to copy curated articles into, default: `/dBranch/curated`

`DBRANCH_PEER_ALLOW_LIST` - a json file with ipfs peer ids to limit whose articles will be curated, see section below for more details, default: `./peer-allow-list.json`

`DBRANCH_ALLOW_EMPTY_PEER_LIST` - if `true` the program will exit if the allow list file does not exist or the list is empty (which implies all articles will be curated), default: `false`

### peer allow list

The peer allow list contains IPFS peer ids that will be automatically curated to the local IPFS node when a new article is received on the `DBRANCH_WIRE_CHANNEL`. If the list is empty all articles will be automatically curated, by default this behaviour is not allowed, see `DBRANCH_ALLOW_EMPTY_PEER_LIST` in the configuration section to override.

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

The structure for the peer list is the following JSON, see configuration section above for setting the file path.

    {
        "allowed_peers": [
            "<peer id>",
            "..."
        ]
    }