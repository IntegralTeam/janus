package transformer

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
	"github.com/qtumproject/janus/pkg/utils"

	"github.com/shopspring/decimal"
)

// ProxyETHSendTransaction implements ETHProxy
type ProxyETHSignTransaction struct {
	*qtum.Qtum
}

func (p *ProxyETHSignTransaction) Method() string {
	return "eth_signTransaction"
}

func (p *ProxyETHSignTransaction) Request(rawreq *eth.JSONRPCRequest) (interface{}, error) {
	var req eth.SendTransactionRequest
	if err := unmarshalRequest(rawreq.Params, &req); err != nil {
		return nil, err
	}

	if req.IsCreateContract() {
		return p.requestCreateContract(&req)
	} else if req.IsSendEther() {
		return p.requestSendToAddress(&req)
	} else if req.IsCallContract() {
		return p.requestSendToContract(&req)
	}

	return nil, errors.New("Unknown operation")
}

func (p *ProxyETHSignTransaction) getRequiredUtxos(from string, neededAmount decimal.Decimal) ([]qtum.RawTxInputs, decimal.Decimal, error) {
	//convert address to qtum address
	addr := utils.RemoveHexPrefix(from)
	base58Addr, err := p.FromHexAddress(addr)
	if err != nil {
		return nil, decimal.Decimal{}, err
	}
	// need to get utxos with txid and vouts. In order to do this we get a list of unspent transactions and begin summing them up
	var unspentListReq *qtum.ListUnspentRequest = &qtum.ListUnspentRequest{MinConf: 6, MaxConf: 9999999, Addresses: []string{base58Addr}}
	qtumresp, err := p.ListUnspent(unspentListReq)
	if err != nil {
		return nil, decimal.Decimal{}, err
	}

	balance := decimal.New(0, 0)
	var inputs []qtum.RawTxInputs
	var balanceReqMet bool
	for _, utxo := range *qtumresp {
		balance = balance.Add(decimal.NewFromFloat(utxo.Amount))
		inputs = append(inputs, qtum.RawTxInputs{TxID: utxo.Txid, Vout: utxo.Vout})
		if balance.GreaterThanOrEqual(neededAmount) {
			balanceReqMet = true
			break
		}
	}
	if balanceReqMet {
		// this is useful for figuring out which utxo was signed as list_unspent seems to be non deterministic
		//fmt.Printf("utxos: %v\n", inputs)
		return inputs, balance, nil
	}
	return nil, decimal.Decimal{}, fmt.Errorf("Insufficient UTXO value attempted to be sent")
}

func calculateChange(balance, neededAmount decimal.Decimal) (decimal.Decimal, error) {
	if balance.LessThan(neededAmount) {
		return decimal.Decimal{}, fmt.Errorf("insufficient funds to create fee to chain")
	}
	return balance.Sub(neededAmount), nil
}

func calculateNeededAmount(value, gasLimit, gasPrice decimal.Decimal) decimal.Decimal {
	return value.Add(gasLimit.Mul(gasPrice))
}

func (p *ProxyETHSignTransaction) requestSendToContract(ethtx *eth.SendTransactionRequest) (string, error) {
	gasLimit, gasPrice, err := EthGasToQtum(ethtx)
	if err != nil {
		return "", err
	}

	amount := 0.0
	if ethtx.Value != "" {
		var err error
		amount, err = EthValueToQtumAmount(ethtx.Value)
		if err != nil {
			return "", errors.Wrap(err, "EthValueToQtumAmount:")
		}
	}

	newGasPrice, err := decimal.NewFromString(gasPrice)
	if err != nil {
		return "", err
	}
	neededAmount := calculateNeededAmount(decimal.NewFromFloat(amount), decimal.NewFromBigInt(gasLimit, 0), newGasPrice)

	inputs, balance, err := p.getRequiredUtxos(ethtx.From, neededAmount)
	if err != nil {
		return "", err
	}

	change, err := calculateChange(balance, neededAmount)
	if err != nil {
		return "", err
	}

	contractInteractTx := &qtum.SendToContractRawRequest{
		ContractAddress: utils.RemoveHexPrefix(ethtx.To),
		Datahex:         utils.RemoveHexPrefix(ethtx.Data),
		Amount:          amount,
		GasLimit:        gasLimit,
		GasPrice:        gasPrice,
	}

	if from := ethtx.From; from != "" && utils.IsEthHexAddress(from) {
		from, err = p.FromHexAddress(from)
		if err != nil {
			return "", err
		}
		contractInteractTx.SenderAddress = from
	}

	fromAddr := utils.RemoveHexPrefix(ethtx.From)

	acc := p.Qtum.Accounts.FindByHexAddress(fromAddr)
	if acc == nil {
		return "", errors.Errorf("No such account: %s", fromAddr)
	}

	rawtxreq := []interface{}{inputs, []interface{}{map[string]*qtum.SendToContractRawRequest{"contract": contractInteractTx}, map[string]decimal.Decimal{contractInteractTx.SenderAddress: change}}}
	var rawTx string
	if err := p.Qtum.Request(qtum.MethodCreateRawTx, rawtxreq, &rawTx); err != nil {
		return "", err
	}

	var resp *qtum.SignRawTxResponse
	if err := p.Qtum.Request(qtum.MethodSignRawTx, rawTx, &resp); err != nil {
		return "", err
	}
	if !resp.Complete {
		return "", fmt.Errorf("something went wrong with signing the transaction; transaction incomplete")
	}
	return utils.AddHexPrefix(resp.Hex), nil
}

func (p *ProxyETHSignTransaction) requestSendToAddress(req *eth.SendTransactionRequest) (string, error) {
	getQtumWalletAddress := func(addr string) (string, error) {
		if utils.IsEthHexAddress(addr) {
			return p.FromHexAddress(utils.RemoveHexPrefix(addr))
		}
		return addr, nil
	}

	to, err := getQtumWalletAddress(req.To)
	if err != nil {
		return "", err
	}

	from, err := getQtumWalletAddress(req.From)
	if err != nil {
		return "", err
	}

	amount, err := EthValueToQtumAmount(req.Value)
	if err != nil {
		return "", err
	}

	neededAmount := decimal.NewFromFloat(amount)
	inputs, balance, err := p.getRequiredUtxos(req.From, neededAmount)
	if err != nil {
		return "", err
	}

	change, err := calculateChange(balance, neededAmount)
	if err != nil {
		return "", err
	}

	var addressValMap = map[string]decimal.Decimal{to: neededAmount, from: change}
	rawtxreq := []interface{}{inputs, addressValMap}
	var rawTx string
	if err := p.Qtum.Request(qtum.MethodCreateRawTx, rawtxreq, &rawTx); err != nil {
		return "", err
	}

	var resp *qtum.SignRawTxResponse
	signrawtxreq := []interface{}{rawTx}
	if err := p.Qtum.Request(qtum.MethodSignRawTx, signrawtxreq, &resp); err != nil {
		return "", err
	}
	if !resp.Complete {
		return "", fmt.Errorf("something went wrong with signing the transaction; transaction incomplete")
	}
	return utils.AddHexPrefix(resp.Hex), nil
}

func (p *ProxyETHSignTransaction) requestCreateContract(req *eth.SendTransactionRequest) (string, error) {
	gasLimit, gasPrice, err := EthGasToQtum(req)
	if err != nil {
		return "", err
	}

	from := req.From
	if utils.IsEthHexAddress(from) {
		from, err = p.FromHexAddress(from)
		if err != nil {
			return "", err
		}
	}

	contractDeploymentTx := &qtum.CreateContractRawRequest{
		ByteCode:      utils.RemoveHexPrefix(req.Data),
		GasLimit:      gasLimit,
		GasPrice:      gasPrice,
		SenderAddress: from,
	}

	newGasPrice, err := decimal.NewFromString(gasPrice)
	if err != nil {
		return "", err
	}
	neededAmount := calculateNeededAmount(decimal.NewFromFloat(0.0), decimal.NewFromBigInt(gasLimit, 0), newGasPrice)

	inputs, balance, err := p.getRequiredUtxos(req.From, neededAmount)
	if err != nil {
		return "", err
	}

	change, err := calculateChange(balance, neededAmount)
	if err != nil {
		return "", err
	}

	rawtxreq := []interface{}{inputs, []interface{}{map[string]*qtum.CreateContractRawRequest{"contract": contractDeploymentTx}, map[string]decimal.Decimal{from: change}}}
	var rawTx string
	if err := p.Qtum.Request(qtum.MethodCreateRawTx, rawtxreq, &rawTx); err != nil {
		return "", err
	}

	var resp *qtum.SignRawTxResponse
	signrawtxreq := []interface{}{rawTx}
	if err := p.Qtum.Request(qtum.MethodSignRawTx, signrawtxreq, &resp); err != nil {
		return "", err
	}
	if !resp.Complete {
		return "", fmt.Errorf("something went wrong with signing the transaction; transaction incomplete")
	}
	return utils.AddHexPrefix(resp.Hex), nil
}
