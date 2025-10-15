package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type Peer struct {
	Id   peer.ID
	Addr []multiaddr.Multiaddr
	Host host.Host
}

const protocol = "/ipv6/1.0.0"

func PeerCreation() (*Peer, error) {
	privkey, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		return nil, fmt.Errorf("error generating keys: %s", err)
	}

	h, err := libp2p.New(
		libp2p.Identity(privkey),
		libp2p.ListenAddrStrings("/ip6/::/tcp/0"), // listen on all IPv6
	)
	if err != nil {
		return nil, fmt.Errorf("error creating host: %s", err)
	}

	return &Peer{
		Id:   h.ID(),
		Addr: h.Addrs(),
		Host: h,
	}, nil
}

func ConnectToPeer(ctx context.Context, h host.Host, peerAddr string) (*peer.AddrInfo, error) {
	maddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid multiaddress: %s", err)
	}

	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, fmt.Errorf("error getting peer info: %s", err)
	}

	if err := h.Connect(ctx, *info); err != nil {
		return nil, fmt.Errorf("connection failed: %s", err)
	}

	fmt.Println("Connected to:", info.ID)
	return info, nil
}

func handleStream(stream network.Stream) {
	fmt.Println("Got a new stream!")

	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer:", err)
			return
		}

		if str != "\n" {
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}
	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading stdin:", err)
			continue
		}

		_, err = rw.WriteString(sendData)
		if err != nil {
			fmt.Println("Error writing to buffer:", err)
			continue
		}

		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer:", err)
		}
	}
}

func main() {
	ctx := context.Background()

	mypeer, err := PeerCreation()
	if err != nil {
		log.Fatal(err)
	}
	defer mypeer.Host.Close()

	fmt.Println("Peer ID:", mypeer.Id)
	fmt.Println("Peer MultiAddresses:")
	for _, addr := range mypeer.Addr {
		fmt.Printf("%s/p2p/%s\n", addr, mypeer.Id)
	}

	mypeer.Host.SetStreamHandler(protocol, handleStream)

	var peerAddr string
	fmt.Print("Enter friend's MultiAddress: ")
	fmt.Scan(&peerAddr)

	info, err := ConnectToPeer(ctx, mypeer.Host, peerAddr)
	if err != nil {
		log.Fatal(err)
	}

	stream, err := mypeer.Host.NewStream(ctx, info.ID, protocol)
	if err != nil {
		log.Fatal("Stream open failed:", err)
	}

	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	go writeData(rw)
	go readData(rw)

	select {}
}
