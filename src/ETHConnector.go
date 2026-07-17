package src

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
)

//struct per i dati del balance API
type ethBalanceResponse struct {
	Result string `json:"result"`
}

//struct per i dati dell'history api
type ethHistory struct{
	Result []struct{
		From string `json:"from"`
		To string `json:"to"`
		Value string `json:"value"`
		TimeStamp string `json:"timestamp"`
		Hash string `json:"hash"`
	} `json:"result"`
}

//convertion wei to eth
func weiToeth(value string)(float64){


	wei := new(big.Int)
	wei.SetString(value,10)

	ethF := new(big.Float).SetInt(wei)
	ethF.Quo(ethF,big.NewFloat(1e18))
	
	result, _ := ethF.Float64()
	return result
}


//Get balance ETH
func (Eth ETHConnector)GetBalance(address string)(float64,error){
	url := "https://eth.blockscout.com/api?module=account&action=balance&address=" + address

	resp, err := http.Get(url)
	if(err != nil){return float64(0), err}

	defer resp.Body.Close()

	var data ethBalanceResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if(err !=nil){ return float64(0), err}

	return weiToeth(data.Result), nil
}

//get history eth
func (Eth ETHConnector)GetTransactions(address string)([]Transfer,error){
	url := fmt.Sprintf("https://eth.blockscout.com/api?module=account&action=txlist&address=%s&sort=desc&page=1&offset=25",address)

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

