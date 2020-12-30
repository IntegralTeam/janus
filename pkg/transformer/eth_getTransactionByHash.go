package transformer

import (
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
	"github.com/qtumproject/janus/pkg/utils"
)

// ProxyETHGetTransactionByHash implements ETHProxy
type ProxyETHGetTransactionByHash struct {
	*qtum.Qtum
}

func (p *ProxyETHGetTransactionByHash) Method() string {
	return "eth_getTransactionByHash"
}

func (p *ProxyETHGetTransactionByHash) Request(rawreq *eth.JSONRPCRequest) (interface{}, error) {
	var req eth.GetTransactionByHashRequest
	if err := unmarshalRequest(rawreq.Params, &req); err != nil {
		return nil, err
	}

	qtumreq := p.ToRequest(&req)

	return p.request(qtumreq)
}

func (p *ProxyETHGetTransactionByHash) request(req *qtum.GetTransactionRequest) (*eth.GetTransactionByHashResponse, error) {
	ethTx, err := GetTransactionByHash(p.Qtum, req.Txid, 0, 0)
	if err != nil {
		return nil, err
	}

	return ethTx, nil
}

func (p *ProxyETHGetTransactionByHash) ToRequest(ethreq *eth.GetTransactionByHashRequest) *qtum.GetTransactionRequest {
	return &qtum.GetTransactionRequest{
		Txid: utils.RemoveHexPrefix(string(*ethreq)),
	}
}
