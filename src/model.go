package src

type Transfer struct{
	From string
	To string
	Amount float64
	Asset string
	Timestamp int64
	Txhash string
	Chain string
}

type Wallet struct{
	Address string
	Type string
	Subtype string
	//more information about wallet
	Balance float64
	History []Transfer
}


//interfaccia
type Connector interface{
	GetBalance(address string)(float64,error)
	GetTransactions(address string)([]Transfer,error)
}

//struct BTC
type BTCconnector struct{}
//struct eth
type ETHConnector struct{}
