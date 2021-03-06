package transformer

import (
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
)

type Transformer struct {
	qtumClient   *qtum.Qtum
	debugMode    bool
	logger       log.Logger
	transformers map[string]ETHProxy
}

// New creates a new Transformer
func New(qtumClient *qtum.Qtum, proxies []ETHProxy, opts ...Option) (*Transformer, error) {
	if qtumClient == nil {
		return nil, errors.New("qtumClient cannot be nil")
	}

	t := &Transformer{
		qtumClient: qtumClient,
		logger:     log.NewNopLogger(),
	}

	var err error
	for _, p := range proxies {
		if err = t.Register(p); err != nil {
			return nil, err
		}
	}

	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// Register registers an ETHProxy to a Transformer
func (t *Transformer) Register(p ETHProxy) error {
	if t.transformers == nil {
		t.transformers = make(map[string]ETHProxy)
	}

	m := p.Method()
	if _, ok := t.transformers[m]; ok {
		return errors.Errorf("method already exist: %s ", m)
	}

	t.transformers[m] = p

	return nil
}

// Transform takes a Transformer and transforms the request from ETH request and returns the proxy request
func (t *Transformer) Transform(rpcReq *eth.JSONRPCRequest) (interface{}, error) {
	p, err := t.getProxy(rpcReq)
	if err != nil {
		return nil, err
	}

	return p.Request(rpcReq)
}

func (t *Transformer) getProxy(rpcReq *eth.JSONRPCRequest) (ETHProxy, error) {
	m := rpcReq.Method
	p, ok := t.transformers[m]
	if !ok {
		return nil, errors.Errorf("Unsupported method %s", m)
	}
	return p, nil
}

// DefaultProxies are the default proxy methods made available
func DefaultProxies(qtumRPCClient *qtum.Qtum) []ETHProxy {
	filter := eth.NewFilterSimulator()
	getFilterChanges := &ProxyETHGetFilterChanges{Qtum: qtumRPCClient, filter: filter}
	ethCall := &ProxyETHCall{Qtum: qtumRPCClient}

	return []ETHProxy{
		ethCall,
		&ProxyNetListening{Qtum: qtumRPCClient},
		&ProxyETHPersonalUnlockAccount{},
		&ProxyETHBlockNumber{Qtum: qtumRPCClient},
		&ProxyETHNetVersion{Qtum: qtumRPCClient},
		&ProxyETHGetTransactionByHash{Qtum: qtumRPCClient},
		&ProxyETHGetLogs{Qtum: qtumRPCClient},
		&ProxyETHGetTransactionReceipt{Qtum: qtumRPCClient},
		&ProxyETHSendTransaction{Qtum: qtumRPCClient},
		&ProxyETHAccounts{Qtum: qtumRPCClient},
		&ProxyETHGetCode{Qtum: qtumRPCClient},

		&ProxyETHNewFilter{Qtum: qtumRPCClient, filter: filter},
		&ProxyETHNewBlockFilter{Qtum: qtumRPCClient, filter: filter},
		getFilterChanges,
		&ProxyETHGetFilterLogs{ProxyETHGetFilterChanges: getFilterChanges},
		&ProxyETHUninstallFilter{Qtum: qtumRPCClient, filter: filter},

		&ProxyETHEstimateGas{ProxyETHCall: ethCall},
		&ProxyETHGetBlockByNumber{Qtum: qtumRPCClient},
		&ProxyETHGetBalance{Qtum: qtumRPCClient},
		&Web3ClientVersion{},
		&ProxyETHSign{Qtum: qtumRPCClient},
		&ProxyETHGasPrice{Qtum: qtumRPCClient},
		&ProxyETHTxCount{Qtum: qtumRPCClient},
		&ProxyETHSignTransaction{Qtum: qtumRPCClient},
		&ProxyETHSendRawTransaction{Qtum: qtumRPCClient},
	}
}

func SetDebug(debug bool) func(*Transformer) error {
	return func(t *Transformer) error {
		t.debugMode = debug
		return nil
	}
}

func SetLogger(l log.Logger) func(*Transformer) error {
	return func(t *Transformer) error {
		t.logger = log.WithPrefix(l, "component", "transformer")
		return nil
	}
}
