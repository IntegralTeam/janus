package transformer

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/qtumproject/janus/pkg/eth"
	"github.com/qtumproject/janus/pkg/qtum"
)

func TestNetVersionRequest(t *testing.T) {
	//preparing request
	requestParams := []json.RawMessage{}
	request, err := prepareEthRPCRequest(1, requestParams)
	if err != nil {
		panic(err)
	}

	mockedClientDoer := doerMappedMock{make(map[string][]byte)}
	qtumClient, err := createMockedClient(mockedClientDoer)
	if err != nil {
		panic(err)
	}

	var getBlockChainInfoResponse qtum.GetBlockChainInfoResponse
	err = unmarshallTestDataFile("testdata/getblockchaininfo1.json", &getBlockChainInfoResponse)
	if err != nil {
		panic(err)
	}

	//preparing client response
	err = mockedClientDoer.AddResponse(2, qtum.MethodGetBlockChainInfo, getBlockChainInfoResponse)
	if err != nil {
		panic(err)
	}

	//preparing proxy & executing request
	proxyEth := ProxyETHNetVersion{qtumClient}
	got, err := proxyEth.Request(request)
	if err != nil {
		panic(err)
	}

	want := eth.NetVersionResponse("0x1024")
	if !reflect.DeepEqual(got, &want) {
		t.Errorf(
			"error\ninput: %s\nwant: %s\ngot: %s",
			request,
			want,
			got,
		)
	}
}
