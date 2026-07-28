package src

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
)

type ethBalanceResponse struct {
	Result string `json:"result"`
}

type ethHistory struct{
	Result []struct{
		From string `json:"from"`
		To string `json:"to"`
		Value string `json:"value"`
		TimeStamp string `json:"timestamp"`
		Hash string `json:"hash"`
	} `json:"result"`
}

// Blockscout returns wei as a decimal string, so it goes through big, never float
func weiToeth(value string)(float64){


	wei := new(big.Int)
	wei.SetString(value,10)

	ethF := new(big.Float).SetInt(wei)
	ethF.Quo(ethF,big.NewFloat(1e18))

	result, _ := ethF.Float64()
	return result
}


func (Eth ETHConnector)GetBalance(address string)(BalanceTxCount,error){

	url := "https://eth.blockscout.com/api?module=account&action=balance&address=" + address

	resp, err := http.Get(url)
	if(err != nil){return BalanceTxCount{}, err}

	defer resp.Body.Close()

	var data ethBalanceResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if(err !=nil){ return BalanceTxCount{}, err}


	// TODO: TxCount is still unset here; the entity resolver will need it
	return BalanceTxCount{
		Balance: weiToeth(data.Result),
	}, nil
}

// Here offset IS the page size, so one call is enough (up to ~10k txs)
func (Eth ETHConnector)GetTransactions(address string, limit int)([]Transfer,error){
	url := fmt.Sprintf("https://eth.blockscout.com/api?module=account&action=txlist&address=%s&sort=desc&page=1&offset=%d",address, limit)

	resp, err := http.Get(url)
	if(err != nil){ return nil, err}

	defer resp.Body.Close()

	var data ethHistory

	err = json.NewDecoder(resp.Body).Decode(&data)
	if(err != nil ){return nil, err}

	var result []Transfer
	for _,object := range data.Result{
		time, _ := strconv.Atoi(object.TimeStamp)
		result = append(result, Transfer{
			From: object.From,
			To: object.To,
			Amount: weiToeth(object.Value),
			Timestamp: int64(time),
			Asset: "ETH",
    		Chain: "ETH",
			Txhash: object.Hash,
		})


	}

	return result, nil
}
