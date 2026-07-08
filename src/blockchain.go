package src

import (
	"errors"
	"strings"
)

type Wallet struct{
	Address string
	Type string
	Subtype string
	//more information about wallet
}

func stringLen(str string)(int){
	var l int
	for range str{
		l++
	}
	return l
}

//base58 checker 
//funzione che scorre i caratteri della stringa e controlla per ogni carattere se si trova in questo alfabeto: 123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz
// niente:  0, O, I, l e altri caratteri
func isBase58(wallet string)(bool){
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, c := range wallet{
		if(!strings.Contains(alphabet,string(c))){
			return false
		}
	}
	return true
	
}

//HEX checker
func isHex(payload string)(bool){
	alphabet := "0123456789abcdefABCDEF"
	for _, c := range payload{
		if(!strings.Contains(alphabet,string(c))){
			return false
		}
	}
	return true
}


func (W *Wallet)Indentification()(error){
	wallet := W.Address

	//check if is a BTC and his sub types
	switch{
	//subtype Legacy P2PKH
	case strings.HasPrefix(wallet, "1") && (stringLen(wallet) >= 24 && stringLen(wallet) <= 34):
		//REMOVE TRASH
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		W.Type = "BTC"
		W.Subtype = "P2PKH"

	case strings.HasPrefix(wallet,"3") && stringLen(wallet) == 34:
		//REMOVE TRASH
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		W.Type = "BTC"
		W.Subtype = "P2SH"
	case strings.HasPrefix(wallet,"bc1q") && (stringLen(wallet) == 42 || stringLen(wallet) == 62):
		W.Type = "BTC"
		W.Subtype = "BECH32" 
	case strings.HasPrefix(wallet, "bc1p") && stringLen(wallet) == 62:
		W.Type = "BTC"
		W.Subtype = "BECH32M"
	//TEST NET
	case (strings.HasPrefix(wallet, "m") || strings.HasPrefix(wallet, "n") || strings.HasPrefix(wallet, "2") || strings.HasPrefix(wallet, "tb1")):
		//REMOVE TRASH
		if(!isBase58(wallet)){
			return errors.New("Wallet is trash: " + wallet)
		}
		W.Type = "BTC"
		W.Subtype = "TEST-NET"

	//check if is EVM
	case stringLen(wallet) == 42 && strings.HasPrefix(wallet,"0x"):
		if(!isHex(wallet[2:])){
			return errors.New("Wallet is invalid: " + wallet)
		}
		W.Type = "EVM"
		W.Subtype = ""


	//check if is dogecoin
	case stringLen(wallet) == 34 && strings.HasPrefix(wallet,"D"):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		W.Type = "DOGE"
		W.Subtype = "P2PKH"
	
	case stringLen(wallet) == 34 && (strings.HasPrefix(wallet,"A") || strings.HasPrefix(wallet,"9")):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		W.Type = "DOGE"
		W.Subtype = "P2SH"

	default:
		W.Type = "NONE"
		W.Subtype = "NONE"
	}
	
	return  nil
}
