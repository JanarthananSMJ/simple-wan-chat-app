package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const protocol = "/ipv6/1.0.0"

// Create a new peer
func createPeer() (host.Host, error) {
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		return nil, err
	}

	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip6/::/tcp/0"),
	)
	if err != nil {
		return nil, err
	}
	return h, nil
}

// Connect to a remote peer
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

// Handle incoming stream
func handleStream(stream network.Stream) {
	fmt.Println("Got a new stream from:", stream.Conn().RemotePeer())
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	go readMessages(rw)
	go writeMessages(rw)
}

// Read messages from stream
func readMessages(rw *bufio.ReadWriter) {
	for {
		msg, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("\nConnection closed:", err)
			return
		}
		msg = strings.TrimSpace(msg)
		if msg != "" {
			fmt.Printf("\rFriend: %s\n> ", msg)
		}
	}
}

// Write messages to stream
func writeMessages(rw *bufio.ReadWriter) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}
		text := scanner.Text() + "\n"
		_, err := rw.WriteString(text)
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
	fmt.Println("MultiAddresses:")
	for _, a := range h.Addrs() {
		fmt.Printf("%s/p2p/%s\n", a, h.ID())
	}

	h.SetStreamHandler(protocol, handleStream)

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter friend's MultiAddress (leave empty to wait for incoming): ")
	peerAddr, _ := reader.ReadString('\n')
	peerAddr = strings.TrimSpace(peerAddr)

	if peerAddr != "" {
		info, err := connectToPeer(ctx, h, peerAddr)
		if err != nil {
			log.Fatal(err)
		}

		stream, err := h.NewStream(ctx, info.ID, protocol)
		if err != nil {
			log.Fatal("Stream open failed:", err)
		}
		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
		go readMessages(rw)
		writeMessages(rw) // this runs on main goroutine
	}

	select {} // keep program running
}
