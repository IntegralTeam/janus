package transformer

import (
	"strings"

	// "github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
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
	var tx *qtum.GetTransactionResponse
	if err := p.Qtum.Request(qtum.MethodGetTransaction, req, &tx); err != nil {
		if err == qtum.EmptyResponseErr {
			return nil, nil
		}

		return nil, err
	}

	ethVal, err := QtumAmountToEthValue(tx.Amount)
	if err != nil {

		return nil, err
	}

	decodedRawTx, err := p.Qtum.DecodeRawTransaction(tx.Hex)
	if err != nil {
		return nil, errors.Wrap(err, "Qtum#DecodeRawTransaction")
	}

	ethTx := eth.GetTransactionByHashResponse{
		Hash:      utils.AddHexPrefix(tx.Txid),
		BlockHash: utils.AddHexPrefix(tx.Blockhash),
		Nonce:     "0x01",
		Value:     ethVal,

		// Contract invokation info:
		// Input,
		// Gas,
		// GasPrice,
	}

	var invokeInfo *qtum.ContractInvokeInfo

	// We assume that this tx is a contract invokation (create or call), if we can
	// find a create or call script.
	for _, out := range decodedRawTx.Vout {
		script := strings.Split(out.ScriptPubKey.Asm, " ")
		finalOp := script[len(script)-1]

		// switch out.ScriptPubKey.Type
		switch finalOp {
		case "OP_CALL":
			info, err := qtum.ParseCallSenderASM(script)
			// OP_CALL with OP_SENDER has the script type "nonstandard"
			if err != nil {
				return nil, err
			}

			invokeInfo = info

			break
		case "OP_CREATE":
			// OP_CALL with OP_SENDER has the script type "create_sender"
			info, err := qtum.ParseCreateSenderASM(script)
			if err != nil {
				return nil, err
			}

			invokeInfo = info

			break
		}
	}

	if invokeInfo != nil {
		ethTx.From = utils.AddHexPrefix(invokeInfo.From)
		ethTx.Gas = utils.AddHexPrefix(invokeInfo.GasLimit) // not really "gas sent by user", but ¯\_(ツ)_/¯
		ethTx.GasPrice = utils.AddHexPrefix(invokeInfo.GasPrice)
		ethTx.Input = utils.AddHexPrefix(invokeInfo.CallData)

		// TODO: Error
		// receipt, err := p.Qtum.GetTransactionReceipt(tx.Txid)
		// if err != nil && err != qtum.EmptyResponseErr {
		// 	return nil, err
		// }

		// if receipt != nil {
		// 	ethTx.BlockNumber = hexutil.EncodeUint64(receipt.BlockNumber)
		// 	ethTx.TransactionIndex = hexutil.EncodeUint64(receipt.TransactionIndex)

		// 	if receipt.ContractAddress != "0000000000000000000000000000000000000000" {
		// 		ethTx.To = utils.AddHexPrefix(receipt.ContractAddress)
		// 	}
		// }
	}

	return &ethTx, nil
}

func (p *ProxyETHGetTransactionByHash) ToRequest(ethreq *eth.GetTransactionByHashRequest) *qtum.GetTransactionRequest {
	return &qtum.GetTransactionRequest{
		Txid: utils.RemoveHexPrefix(string(*ethreq)),
	}
}
