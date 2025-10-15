package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const protocol = "/ipv6/1.0.0"

var (
	activeStream network.Stream
	streamMutex  sync.Mutex
)

// create a new peer host
func createPeer() (host.Host, error) {
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		return nil, err
	}
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip6/::/tcp/0"), // listen on all IPv6
	)
	if err != nil {
		return nil, err
	}
	return h, nil
}

// connect to remote peer
func connectToPeer(ctx context.Context, h host.Host, addr string) (*peer.AddrInfo, error) {
	maddr, err := multiaddr.NewMultiaddr(strings.TrimSpace(addr))
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

// handle incoming stream
func handleStream(stream network.Stream) {
	fmt.Println("\nGot a new stream from:", stream.Conn().RemotePeer())

	streamMutex.Lock()
	activeStream = stream
	streamMutex.Unlock()

	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	go readMessages(rw)
}

// read messages from stream
func readMessages(rw *bufio.ReadWriter) {
	for {
		msg, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("\nConnection closed:", err)
			streamMutex.Lock()
			activeStream = nil
			streamMutex.Unlock()
			return
		}
		msg = strings.TrimSpace(msg)
		if msg != "" {
			fmt.Printf("\rFriend: %s\n> ", msg)
		}
	}
}

// write messages - this runs continuously in main
func writeMessages() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}
		text := scanner.Text()
		if text == "" {
			continue
		}

		streamMutex.Lock()
		stream := activeStream
		streamMutex.Unlock()

		if stream == nil {
			fmt.Println("Not connected to any peer yet")
			continue
		}

		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
		_, err := rw.WriteString(text + "\n")
		if err != nil {
			fmt.Println("Error writing:", err)
			continue
		}
		rw.Flush()
	}
}

func main() {
	ctx := context.Background()
	h, err := createPeer()
	if err != nil {
		log.Fatal(err)
	}
	defer h.Close()

	fmt.Println("Peer ID:", h.ID())
	fmt.Println("Peer MultiAddresses:")
	for _, a := range h.Addrs() {
		fmt.Printf("%s/p2p/%s\n", a, h.ID())
	}

	// handle incoming streams
	h.SetStreamHandler(protocol, handleStream)

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nEnter friend's MultiAddress (leave empty to wait for incoming): ")
	peerAddr, _ := reader.ReadString('\n')
	peerAddr = strings.TrimSpace(peerAddr)

	// if address provided, dial the peer
	if peerAddr != "" {
		info, err := connectToPeer(ctx, h, peerAddr)
		if err != nil {
			log.Fatal(err)
		}
		stream, err := h.NewStream(ctx, info.ID, protocol)
		if err != nil {
			log.Fatal("Stream open failed:", err)
		}

		streamMutex.Lock()
		activeStream = stream
		streamMutex.Unlock()

		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
		go readMessages(rw)
	} else {
		fmt.Println("Waiting for incoming connection...")
	}

	// Both peers can now send and receive
	writeMessages()
}
