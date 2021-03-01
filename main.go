package main

import (
	tokenhub "MainsiteDeposit/abi"
	"MainsiteDeposit/utils"
	"context"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/go-sdk/common/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"io/ioutil"
	"math/big"
	"strings"
	"sync"
)

var wsEndpoint = "wss://bsc-ws-node.nariox.org:443"
var tokenHub = common.HexToAddress("0x0000000000000000000000000000000000001004")

func main() {
	client, _ := ethclient.Dial(wsEndpoint)
	mc, _ := tokenhub.NewTokenhub(tokenHub, client)
	go calTransferOutResult(client, mc)
	go calFefundResult(client, mc)
	select {}
}

func calTransferOutResult(c *ethclient.Client, mc *tokenhub.Tokenhub) {

	startCalHeight := uint64(1)
	finalCalHeight := uint64(5306245)
	hubAPI, err := abi.JSON(strings.NewReader(tokenhub.TokenhubABI))
	if err != nil {
		panic("marshal abi error")
	}
	var endHeight uint64
	taskPool := utils.NewPool(50, 50, 10)
	transferOuts := make([]*TransferOut, 0)
	mx := sync.Mutex{}
	for height := startCalHeight; height <= finalCalHeight; {
		fmt.Printf("syncing height %d \n", height)
		endHeight = height + 4000

		txs, err := GetTransoutTxs(mc, big.NewInt(int64(height)), big.NewInt(int64(endHeight)))
		if err != nil {
			panic(err)
		}

		var wg sync.WaitGroup
		wg.Add(len(txs))
		for tx, event := range txs {
			tmpTxHash := tx
			taskPool.Schedule(func() {
				tmpTx, _, err := c.TransactionByHash(context.Background(), tmpTxHash)
				if err != nil {
					fmt.Println(err)
				} else {
					arg, err := upPackTx(tmpTx.Data(), &hubAPI)
					if err != nil {
						fmt.Println("unpack failed")
						fmt.Println(err)
						return
					}
					bz := arg.Recipient.Bytes()
					addr := types.AccAddress(bz)
					if addr.String() == "bnb136ns6lfw4zs5hg4n85vdthaad7hq5m4gtkgf23" {
						mx.Lock()
						fmt.Printf("find  match %s\n", tmpTx.Hash().String())
						transferOuts = append(transferOuts, &TransferOut{
							Hegiht:        event.Raw.BlockNumber,
							TxHash:        tmpTxHash,
							Sender:        event.SenderAddr,
							Bep20Contract: event.Bep20Addr,
							Recipient:     arg.Recipient,
							Amount:        arg.Amount,
							ExpireTime:    arg.ExpireTime,
						})
						mx.Unlock()
					} else {
						fmt.Printf("find  not match %s\n", tmpTx.Hash().String())
					}
				}
				wg.Done()
				return
			})
		}
		wg.Wait()
		height = endHeight + 1
	}
	bz, _ := json.MarshalIndent(transferOuts, "", "\t")
	ioutil.WriteFile(fmt.Sprintf("transfer_out.json"), bz, 0600)
	fmt.Println("done transfer_out")

}

func calFefundResult(c *ethclient.Client, mc *tokenhub.Tokenhub) {

	startCalHeight := uint64(1)
	finalCalHeight := uint64(5306245)
	refunds := make([]*Refund, 0)
	var endHeight uint64
	for height := startCalHeight; height <= finalCalHeight; {
		fmt.Printf("syncing height %d \n", height)
		endHeight = height + 4000

		txs, err := GetRefundTxs(mc, big.NewInt(int64(height)), big.NewInt(int64(endHeight)))
		if err != nil {
			panic(err)
		}

		var wg sync.WaitGroup
		wg.Add(len(txs))
		for tx, event := range txs {
			refunds = append(refunds, &Refund{
				Hegiht:        event.Raw.BlockNumber,
				TxHash:        tx,
				Bep20Contract: event.Bep20Addr,
				RefundAddr:    event.RefundAddr,
				Amount:        event.Amount,
			})

		}
		wg.Wait()
		height = endHeight + 1
	}
	bz, _ := json.MarshalIndent(refunds, "", "\t")
	ioutil.WriteFile(fmt.Sprintf("refund.json"), bz, 0600)
	fmt.Println("done refund")
}

func GetRefundTxs(mc *tokenhub.Tokenhub, start, end *big.Int) (map[common.Hash]*tokenhub.TokenhubRefundSuccess, error) {
	res := make(map[common.Hash]*tokenhub.TokenhubRefundSuccess, 0)
	e := end.Uint64()
	ite1, err := mc.FilterRefundSuccess(&bind.FilterOpts{
		Start: start.Uint64(),
		End:   &e,
	})
	if err != nil {
		return nil, err
	}
	for ite1.Next() {
		res[ite1.Event.Raw.TxHash] = ite1.Event
	}
	return res, nil
}

func GetTransoutTxs(mc *tokenhub.Tokenhub, start, end *big.Int) (map[common.Hash]*tokenhub.TokenhubTransferOutSuccess, error) {
	res := make(map[common.Hash]*tokenhub.TokenhubTransferOutSuccess, 0)
	e := end.Uint64()
	ite1, err := mc.FilterTransferOutSuccess(&bind.FilterOpts{
		Start: start.Uint64(),
		End:   &e,
	})
	if err != nil {
		return nil, err
	}
	for ite1.Next() {
		res[ite1.Event.Raw.TxHash] = ite1.Event
	}
	return res, nil
}

type TransferOut struct {
	Hegiht        uint64
	TxHash        common.Hash
	Sender        common.Address
	Bep20Contract common.Address
	Recipient     common.Address
	Amount        *big.Int
	ExpireTime    uint64
}

type Refund struct {
	Hegiht        uint64
	TxHash        common.Hash
	Bep20Contract common.Address
	RefundAddr    common.Address
	Amount        *big.Int
}

type Args struct {
	ContractAddr common.Address
	Recipient    common.Address
	Amount       *big.Int
	ExpireTime   uint64
}

func upPackTx(bzs []byte, hubABI *abi.ABI) (*Args, error) {
	method, err := hubABI.MethodById(bzs[:4])
	if err != nil {
		return nil, fmt.Errorf("fail to parse")
	}

	if method == nil {
		return nil, fmt.Errorf("method  is nil")
	}
	if method.Name != "transferOut" {
		return nil, fmt.Errorf("Not transfer out")
	}

	var ifc = Args{}

	err = method.Inputs.Unpack(&ifc, bzs[4:])
	if err != nil {
		return nil, err
	}
	return &ifc, nil
}
