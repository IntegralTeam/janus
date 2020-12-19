package transformer

import (
	"encoding/json"
	"fmt"
	"math/big"

	"strings"

	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"github.com/qtumproject/janus/pkg/utils"
)

type EthGas interface {
	GasHex() string
	GasPriceHex() string
}

func EthGasToQtum(g EthGas) (gasLimit *big.Int, gasPrice string, err error) {
	gasLimit = g.(*eth.SendTransactionRequest).Gas.Int

	gasPriceFloat64, err := EthValueToQtumAmount(g.GasPriceHex())
	if err != nil {
		return nil, "0.0", err
	}
	gasPrice = fmt.Sprintf("%.8f", gasPriceFloat64)

	return
}

func EthValueToQtumAmount(val string) (float64, error) {
	if val == "" {
		return 0.0000004, nil
	}

	ethVal, err := utils.DecodeBig(val)
	if err != nil {
		return 0.0, err
	}

	ethValFloat64 := new(big.Float)
	ethValFloat64, success := ethValFloat64.SetString(ethVal.String())
	if !success {
		return 0.0, errors.New("big.Float#SetString is not success")
	}

	amount := ethValFloat64.Mul(ethValFloat64, big.NewFloat(float64(1e-8)))
	result, _ := amount.Float64()

	return result, nil
}

func QtumAmountToEthValue(amount float64) (string, error) {
	bigAmount := big.NewFloat(amount)
	bigAmount = bigAmount.Mul(bigAmount, big.NewFloat(float64(1e8)))

	result := new(big.Int)
	result, success := result.SetString(bigAmount.Text('f', 0), 10)
	if !success {
		return "0x0", errors.New("big.Int#SetString is not success")
	}

	return hexutil.EncodeBig(result), nil
}

func unmarshalRequest(data []byte, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		return errors.Wrap(err, "Invalid RPC input")
	}
	return nil
}

func GetTransactionByHash(p *qtum.Qtum, hash string, height, position int) (*eth.GetTransactionByHashResponse, error) {
	txData, err := p.GetTransaction(hash)
	if err != nil {
		raw, err := p.GetRawTransaction(hash)

		if err != nil {
			return nil, err
		}

		txData = &qtum.GetTransactionResponse{
			Amount:            0.0,
			Fee:               0.0,
			Confirmations:     raw.Confirmations,
			Blockhash:         raw.Blockhash,
			Blockindex:        0,
			Blocktime:         raw.Blocktime,
			Txid:              raw.Txid,
			Time:              raw.Time,
			Timereceived:      0,
			Bip125Replaceable: "",
			Details:           []*qtum.TransactionDetail{},
			Hex:               raw.Hex,
		}
	}

	ethVal, err := QtumAmountToEthValue(txData.Amount)
	if err != nil {
		return nil, err
	}

	decodedRawTx, err := p.DecodeRawTransaction(txData.Hex)
	if err != nil {
		return nil, errors.Wrap(err, "Qtum#DecodeRawTransaction")
	}

	/// TODO: Correct to normal values
	ethTx := eth.GetTransactionByHashResponse{
		Hash:             utils.AddHexPrefix(txData.Txid),
		Nonce:            "0x01",
		BlockHash:        utils.AddHexPrefix(txData.Blockhash),
		BlockNumber:      hexutil.EncodeUint64(uint64(height)),
		TransactionIndex: hexutil.EncodeUint64(uint64(position)),
		From:             "0x0000000000000000000000000000000000000000",
		To:               "0x0000000000000000000000000000000000000000",
		Value:            ethVal,
		GasPrice:         hexutil.EncodeUint64(uint64(txData.Fee)),
		Gas:              "0x01",
		Input:            "0x00",
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

		// receipt, err := p.Qtum.GetTransactionReceipt(txData.Txid)
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
