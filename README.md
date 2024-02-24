# Skypier VPN

decentralized VPN POC

## Build and Run (on Linux)

```bash
go build -o build/skypier-vpn-node cmd/skypier-vpn-node/main.go
sudo ./build/skypier-vpn-node
```

## TODO list

- [x] Init libp2p node
- [x] Init REST API (Gin)
- [x] Read config file
- [x] Init TUN interface on Linux
- [ ] Init TUN interface on Darwin
- [ ] Init TUN interface on Windows
- [x] If "privateKey" is given in the config, bootsrap the node using this private key
- [x] Launch web UI at launch
- [x] Enable QUIC protocol 
- [ ] Add some dynamic frontend build using Github Action
- [ ] Ensure QUIC protocol first (need more test)
- [ ] Check remote peer is reachable
- [ ] Mount tunnel with remote address corresponding to this peer ID (check the DHT)
- [ ] Config validated with a JsonSchema 
