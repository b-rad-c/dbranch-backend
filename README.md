# dBranch Backend

The backend code for the dbranch node. It is a work in progress and will have more features added over time. Right now its functionality is limited to listening to an IPFS pubsub channel for new articles and then copying them to the local ipfs node's curated directory. This is slightly different than pinning files because the publisher does not have to pin the file for a curator to host it.

### configuration
Configuration is done through the following environment variables:
`IPFS_HOST` - the address of the local ipfs node, default: `localhost:5001`
`DBRANCH_WIRE_CHANNEL` - the IPFS pubsub topic to listen for new articles on, default: `dbranch-wire`
`DBRANCH_CURATED_DIRECTORY` - the IPFS files directory to copy curated articles into, default: `/dBranch/curated`