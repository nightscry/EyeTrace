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
	Balance float64
	History []Transfer
}


// Every connector must implement this: changing a signature breaks all of them.
type Connector interface{
	GetBalance(address string)(BalanceTxCount,error)
	GetTransactions(address string, limit int)([]Transfer,error)
}


// TxCount is the wallet's total tx count, kept for the entity resolver.
type BalanceTxCount struct{
	Balance float64
	TxCount int
}


type BTCconnector struct{}
type ETHConnector struct{}
type DOGEConnector struct{}
