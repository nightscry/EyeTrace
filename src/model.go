package src

import "errors"

const Version = "1.0.0"

type Transfer struct{
	From string `json:"from"`
	To string `json:"to"`
	Amount float64 `json:"amount"`
	Asset string `json:"asset"`
	Timestamp int64 `json:"timestamp"`
	Txhash string `json:"txhash"`
	Chain string `json:"chain"`
}

type Wallet struct{
	Address string
	Type string
	Subtype string
	Testnet bool
	Balance float64
	History []Transfer
}


// Every connector must implement this: changing a signature breaks all of them.
type Connector interface{
	GetBalance(address string)(BalanceTxCount,error)
	GetTransactions(address string, limit int)([]Transfer,error)
}


// TxCount is the wallet's total tx count, 0 when the explorer does not expose it.
// Name and IsContract are what the explorer itself knows about the address
// (a verified contract name, for instance); empty where the chain has no such data.
type BalanceTxCount struct{
	Balance float64
	TxCount int
	Name string
	IsContract bool
}


type BTCconnector struct{ Testnet bool }
type ETHConnector struct{}
type DOGEConnector struct{}
type TronConnector struct{}
type XRPConnector struct{}
type SolanaConnector struct{}


// Picks the connector for a wallet already run through Indentification.
func ConnectorFor(w Wallet) (Connector, error) {
	switch w.Type {
	case "BTC":
		return BTCconnector{Testnet: w.Testnet}, nil
	case "ETH":
		return ETHConnector{}, nil
	case "DOGE":
		return DOGEConnector{}, nil
	case "TRON":
		return TronConnector{}, nil
	case "XRP":
		return XRPConnector{}, nil
	case "SOLANA":
		return SolanaConnector{}, nil
	}
	return nil, errors.New("no connector for chain " + w.Type)
}


// Appended to a token symbol whose contract is not a known one: spam and
// address-poisoning tokens often reuse real symbols like "USDT".
const UnverifiedMark = "?"

// Unit the chain's balance is expressed in (wl.Type is the chain, not the coin).
func NativeAsset(chain string) string {
	switch chain {
	case "TRON":
		return "TRX"
	case "SOLANA":
		return "SOL"
	}
	return chain
}

// Public explorer page for an address, for the "open in explorer" button.
func ExplorerURL(chain string, testnet bool, address string) string {
	switch chain {
	case "BTC":
		if testnet {
			return "https://blockstream.info/testnet/address/" + address
		}
		return "https://blockstream.info/address/" + address
	case "ETH":
		return "https://eth.blockscout.com/address/" + address
	case "DOGE":
		return "https://blockchair.com/dogecoin/address/" + address
	case "TRON":
		return "https://tronscan.org/#/address/" + address
	case "XRP":
		return "https://livenet.xrpl.org/accounts/" + address
	case "SOLANA":
		return "https://explorer.solana.com/address/" + address
	}
	return ""
}
