package vpn

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"time"

	b64 "encoding/base64"
	"encoding/binary"

	"github.com/SkyPierIO/skypier-vpn/pkg/utils"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	peerstore "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	noise "github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"github.com/songgao/water"
	"golang.org/x/net/ipv4"
)

var (
	tunEnabled bool
	nodeIface  *water.Interface
)

func displayNodeInfo(node host.Host) {
	// print node ID
	log.Println("───────────────────────────────────────────────────")
	log.Println("libp2p peer ID: ", node.ID())

	// print the node's PeerInfo in multiaddr format
	peerInfo := peerstore.AddrInfo{
		ID:    node.ID(),
		Addrs: node.Addrs(),
	}
	addrs, err := peerstore.AddrInfoToP2pAddrs(&peerInfo)
	utils.Check(err)

	log.Println("libp2p peer address:")
	for i := 0; i < len(addrs); i++ {
		log.Println("\t", addrs[i])
	}
	log.Println("───────────────────────────────────────────────────")
}

func StartNode(innerConfig utils.InnerConfig, pk crypto.PrivKey, tcpPort string, udpPort string) (host.Host, *dht.IpfsDHT, error) {
	// Init a libp2p node
	// ----------------------------------------------------------

	// The context governs the lifetime of the libp2p node.
	// Cancelling it will stop the host.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connection manager - Load Balancer
	connmgr, err := connmgr.NewConnManager(
		100,  // Lowwater
		8000, // HighWater,
		connmgr.WithGracePeriod(time.Minute),
	)
	utils.Check(err)

	// Sometimes the swarm_stream is left open, but the underlying yamux_stream is closed.
	// This causes the resource limit to be reached. We Need to add monitoring and force to close old streams
	resourceManager, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits))
	utils.Check(err)

	var idht *dht.IpfsDHT

	// QUIC is an UDP-based transport protocol.
	// QUIC connections are always encrypted (using TLS 1.3) and
	// provides native stream multiplexing.
	// Whenever possible, QUIC should be preferred over TCP.
	// Not only is it faster, it also increases the chances of a
	// successful holepunch in case of firewalls

	// However: UDP is blocked in ~5-10% of networks,
	// especially in corporate networks, so running a node
	// exclusively with QUIC is usually not an option.

	// TODO add a cli/config option to prevent private IP advertising
	node, err := libp2p.New(
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			// "/ip6/::/udp/"+udpPort+"/quic-v1",      // IPv6 QUIC
			"/ip4/0.0.0.0/udp/"+udpPort+"/quic-v1", // IPv4 QUIC
			// "/ip6/::/tcp/"+tcpPort,                 // IPv6 TCP
			"/ip4/0.0.0.0/tcp/"+tcpPort, // IPv4 TCP
		),
		// Use the keypair we generated / from config file
		libp2p.Identity(pk),
		// Enable stream connection multiplexers
		libp2p.DefaultMuxers,
		// libp2p.DefaultSecurity,
		// support TLS connections
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		// support noise connections
		libp2p.Security(noise.ID, noise.New),
		// support QUIC transports
		libp2p.Transport(quic.NewTransport),
		// support default TCP transport
		libp2p.Transport(tcp.NewTCPTransport),
		// Let's prevent our peer from having too many
		// connections by attaching a connection manager.
		libp2p.ConnectionManager(connmgr),
		// Attempt to open ports using uPNP for NATed hosts.
		libp2p.NATPortMap(),
		// Let this host use the DHT to find other hosts
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			idht, err = dht.New(ctx, h)
			return idht, err
		}),
		// add monitoring and force to close old streams
		libp2p.ResourceManager(resourceManager),
		libp2p.FallbackDefaults,
		libp2p.Ping(true),
	)
	utils.Check(err)
	// defer node.Close()

	keyBytes, err := crypto.MarshalPrivateKey(node.Peerstore().PrivKey(node.ID()))
	utils.Check(err)
	sEnc := b64.StdEncoding.EncodeToString([]byte(keyBytes))
	if utils.IsDebugEnabled() {
		log.Println(sEnc)
	}

	// // Start a DHT, for use in peer discovery. We can't just make a new DHT client
	// // because we want each peer to maintain its own local copy of the DHT, so
	// // that the bootstrapping node of the DHT can go down without inhibitting
	// // future peer discovery.
	// //
	// // Use dht.NewDHTClient if you don't want our DHT to be requested
	// newDHT := dht.NewDHT(ctx, node, datastore.NewMapDatastore())

	// // Bootstrap the DHT. In the default configuration, this spawns a Background
	// // thread that will refresh the peer table every five minutes.
	// if err = newDHT.Bootstrap(ctx); err != nil {
	// 	log.Fatal(err)
	// }

	// Dev test bootstrap node (NL)
	// TODO add more bootstrap nodes for Skypier in other countries to avoid single point of failure
	// TODO add some bootstrap nodes with TCP && QUIC
	// TODO avoid having default bootstrap nodes hardcoded here. could be get from an online URI, easier for future update

	ipfsPublicPeer, err := multiaddr.NewMultiaddr("/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb")
	utils.Check(err)
	skypierPublicPeer, err := multiaddr.NewMultiaddr("/ip4/136.244.105.166/udp/4001/quic-v1/p2p/12D3KooWKzmZmLySs5WKBvdxzsctWNsN9abbtnj4PyyqNg9LCyek")
	utils.Check(err)
	skypierBootstrapPeers := [...]multiaddr.Multiaddr{
		skypierPublicPeer,
		ipfsPublicPeer,
	}

	// This connects to public bootstrappers
	// use `dht.DefaultBootstrapPeers` for IPFS public bootstrap nodes
	for _, addr := range skypierBootstrapPeers {
		pi, _ := peerstore.AddrInfoFromP2pAddr(addr)
		// We ignore errors as some bootstrap peers may be down
		// and that is fine.
		err := node.Connect(ctx, *pi)
		utils.Check(err)
		log.Println("Connected to bootstrap peer: ", pi.ID)
	}

	log.Println("Enabling Stream Handler...")
	// Set the Skypier protocol handler on the Host's Mux
	node.SetStreamHandler("/skypier/1.0", streamHandler)
	log.Println("Stream handler enabled for protocol /skypier/1.0")

	return node, idht, err
}

func SetNodeUp(ctx context.Context, config utils.InnerConfig) (host.Host, *dht.IpfsDHT) {
	log.Println("Generating identity...")
	privKey, err := loadPrivateKey()
	utils.Check(err)

	// Find available port for both TCP and UDP
	tcpPort := utils.GetFirstAvailableTCPPort(4002, 4999)
	udpPort := utils.GetFirstAvailableTCPPort(4002, 4999)

	node, dht, err := StartNode(config, privKey, tcpPort, udpPort)
	utils.Check(err)
	displayNodeInfo(node)

	return node, dht
}

func streamHandler(s network.Stream) {
	log.Println("Entered the stream handler...")
	log.Println("node status", utils.IS_NODE_HOST)

	// Create a buffer stream for non-blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	go readData(rw)
	go writeData(rw)

	// stream will stay open until you close it (or the other side closes it).
}

func writeData(rw *bufio.ReadWriter) {
	// if !tunEnabled {
	// 	nodeIface = SetInterfaceUp()
	// 	tunEnabled = true
	// }
	// for {
	// 	packet := make([]byte, 1420)
	// 	// packetSize := make([]byte, 2)
	// 	_, err := nodeIface.Read(packet)
	// 	if err != nil {
	// 		fmt.Println(err)
	// 	}
	fmt.Println("Reading from TUN...")
	// 	fmt.Println(packet)
	// 	fmt.Println("Writing on the stream...")
	_, err := rw.WriteString("Hello")
	if err != nil {
		// return
		log.Fatal(err)
	}
	// 	err = rw.Flush()
	// 	if err != nil {
	// 		return
	// 	}
	// }
}

func readData(rw *bufio.ReadWriter) {
	packet := make([]byte, 1420)
	packetSize := make([]byte, 2)
	for {
		// Read the incoming packet's size as a binary value.
		_, err := rw.Read(packetSize)
		if err != nil {
			// stream.Close()
			return
		}

		// Decode the incoming packet's size from binary.
		size := binary.LittleEndian.Uint16(packetSize)
		log.Println("receiving packet of size", size)

		// Read in the packet until completion.
		var plen uint16 = 0
		for plen < size {
			tmp, err := rw.Read(packet[plen:size])
			plen += uint16(tmp)
			if err != nil {
				// stream.Close()
				return
			}
		}

		if utils.IS_NODE_HOST {
			fmt.Println("IS A NODE -- DEBUG")
			fmt.Println(rw)
			if !tunEnabled {
				nodeIface = SetInterfaceUp()
				tunEnabled = true
			}
			fmt.Println("───────────────────── IP packet ─────────────────────")
			// debug
			header, _ := ipv4.ParseHeader(packet[:plen])
			fmt.Printf("Reading IP packet: %+v (%+v)\n", header, err)
			proto := utils.GetProtocolById(packet[9])
			fmt.Println("Protocol:\t", proto)
			src := net.IPv4(packet[12], packet[13], packet[14], packet[15]).String()
			fmt.Println("Source:\t\t", src)
			dst := net.IPv4(packet[16], packet[17], packet[18], packet[19]).String()
			fmt.Println("Destination:\t", dst)
			fmt.Println("─────────────────────────────────────────────────────")

			_, err = nodeIface.Write(packet[:size])
			utils.Check(err)
			// _, err = rw.Write(packet[:size])
			// utils.Check(err)
			// log.Println("writing response on stream, size in bytes:", size)

		} else {
			fmt.Println("IS A CLIENT PEER -- DEBUG")
		}
	}
}
