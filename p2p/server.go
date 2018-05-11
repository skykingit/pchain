package p2p

import (
	cmn "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-p2p"
	"strings"
	//"github.com/tendermint/tendermint/state/txindex/null"
	"github.com/tendermint/go-rpc"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/consensus"
	rpccore "github.com/tendermint/tendermint/rpc/core"
	"github.com/tendermint/tendermint/version"
)

type PChainP2P struct {
	privKey  crypto.PrivKeyEd25519 // local node's p2p key
	sw       *p2p.Switch           // p2p connections
	addrBook *p2p.AddrBook         // known peer
}

func StartP2P(p2pconfig cfg.Config) (*PChainP2P, error) {

	// Generate node PrivKey
	privKey := crypto.GenPrivKeyEd25519()

	// Make p2p network switch
	sw := p2p.NewSwitch(p2pconfig.GetConfig("p2p"))

	// Optionally, start the pex reactor
	var addrBook *p2p.AddrBook
	if p2pconfig.GetBool("pex_reactor") {
		addrBook = p2p.NewAddrBook(p2pconfig.GetString("addrbook_file"), p2pconfig.GetBool("addrbook_strict"))
		pexReactor := p2p.NewPEXReactor(addrBook)
		sw.AddReactor("pchain", "PEX", pexReactor)
	}

	// Filter peers by addr or pubkey with an ABCI query.
	// If the query return code is OK, add peer.
	// XXX: Query format subject to change
	if p2pconfig.GetBool("filter_peers") {
		// NOTE: addr is ip:port
		//sw.SetAddrFilter(func(addr net.Addr) error {
		//	resQuery, err := n.proxyApp.Query().QuerySync(abci.RequestQuery{Path: cmn.Fmt("/p2p/filter/addr/%s", addr.String())})
		//	if err != nil {
		//		return err
		//	}
		//	if resQuery.Code.IsOK() {
		//		return nil
		//	}
		//	return errors.New(resQuery.Code.String())
		//})
		//sw.SetPubKeyFilter(func(pubkey crypto.PubKeyEd25519) error {
		//	resQuery, err := n.proxyApp.Query().QuerySync(abci.RequestQuery{Path: cmn.Fmt("/p2p/filter/pubkey/%X", pubkey.Bytes())})
		//	if err != nil {
		//		return err
		//	}
		//	if resQuery.Code.IsOK() {
		//		return nil
		//	}
		//	return errors.New(resQuery.Code.String())
		//})
	}

	// Create & add listener
	protocol, address := ProtocolAndAddress(p2pconfig.GetString("node_laddr"))
	l := p2p.NewDefaultListener(protocol, address, p2pconfig.GetBool("skip_upnp"))
	sw.AddListener(l)

	// Start the switch
	sw.SetNodeInfo(makeNodeInfo(p2pconfig, privKey, sw))
	sw.SetNodePrivKey(privKey)
	_, err := sw.Start()
	if err != nil {
		return nil, err
	}

	// If seeds exist, add them to the address book and dial out
	if p2pconfig.GetString("seeds") != "" {
		// dial out
		seeds := strings.Split(p2pconfig.GetString("seeds"), ",")
		if err := sw.DialSeeds(addrBook, seeds); err != nil {
			return nil, err
		}
	}
	return &PChainP2P{
		privKey:  privKey,
		sw:       sw,
		addrBook: addrBook,
	}, nil
}

func (pNode *PChainP2P) StopP2P() {
	//log.Notice("Stopping Node")
	// TODO: gracefully disconnect from peers.
	pNode.sw.Stop()
}

// Switch return the P2P Switch
func (pNode *PChainP2P) Switch() *p2p.Switch {
	return pNode.sw
}

// AddrBook return the P2P Address Book
func (pNode *PChainP2P) AddrBook() *p2p.AddrBook {
	return pNode.addrBook
}

func makeNodeInfo(p2pconfig cfg.Config, privKey crypto.PrivKeyEd25519, sw *p2p.Switch) *p2p.NodeInfo {

	//txIndexerStatus := "on"
	//if _, ok := n.txIndexer.(*null.TxIndex); ok {
	//	txIndexerStatus = "off"
	//}

	nodeInfo := &p2p.NodeInfo{
		PubKey:   privKey.PubKey().(crypto.PubKeyEd25519),
		Moniker:  p2pconfig.GetString("moniker"),
		Networks: p2p.MakeNetwork(),
		Version:  version.Version,
		Other: []string{
			cmn.Fmt("wire_version=%v", wire.Version),
			cmn.Fmt("p2p_version=%v", p2p.Version),
			cmn.Fmt("consensus_version=%v", consensus.Version),
			cmn.Fmt("rpc_version=%v/%v", rpc.Version, rpccore.Version),
			//cmn.Fmt("tx_index=%v", txIndexerStatus),
		},
	}

	if !sw.IsListening() {
		return nodeInfo
	}

	p2pListener := sw.Listeners()[0]
	p2pHost := p2pListener.ExternalAddress().IP.String()
	p2pPort := p2pListener.ExternalAddress().Port
	rpcListenAddr := p2pconfig.GetString("rpc_laddr")

	// We assume that the rpcListener has the same ExternalAddress.
	// This is probably true because both P2P and RPC listeners use UPnP,
	// except of course if the rpc is only bound to localhost
	nodeInfo.ListenAddr = cmn.Fmt("%v:%v", p2pHost, p2pPort)
	nodeInfo.Other = append(nodeInfo.Other, cmn.Fmt("rpc_addr=%v", rpcListenAddr))
	return nodeInfo
}

// Defaults to tcp
func ProtocolAndAddress(listenAddr string) (string, string) {
	protocol, address := "tcp", listenAddr
	parts := strings.SplitN(address, "://", 2)
	if len(parts) == 2 {
		protocol, address = parts[0], parts[1]
	}
	return protocol, address
}