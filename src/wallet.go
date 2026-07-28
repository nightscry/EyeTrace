package src

import (
	"errors"
	"strings"
)




func (W *Wallet)Indentification()(error){
	wallet := W.Address

	switch{
	// BTC
	case strings.HasPrefix(wallet, "1") && (stringLen(wallet) >= 24 && stringLen(wallet) <= 34):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet,alphabetBase58)){
			W.Type = "BTC"
			W.Subtype = "P2PKH"
		}else{
			return errors.New("Invalid checksum: "+ wallet)
		}



	case strings.HasPrefix(wallet,"3") && stringLen(wallet) == 34:
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet,alphabetBase58)){
			W.Type = "BTC"
			W.Subtype = "P2SH"
		}else{
			return errors.New("Invalid checksum: "+ wallet)
		}


	case (strings.HasPrefix(wallet,"bc1q") || strings.HasPrefix(wallet, "tb1q"))&& (stringLen(wallet) == 42 || stringLen(wallet) == 62):

		if(checkSumBech(wallet)){
			W.Type = "BTC"
			W.Subtype = "BECH32"
		}else{
			return errors.New("Wallet is invalid: " + wallet)
		}

	case (strings.HasPrefix(wallet, "bc1p") || strings.HasPrefix(wallet, "tb1p"))&& stringLen(wallet) == 62:

		if(checkSumBech(wallet)){
			W.Type = "BTC"
			W.Subtype = "BECH32M"
		}else{
			return errors.New("Wallet is invalid: " + wallet)
		}

	// BTC testnet
	case (strings.HasPrefix(wallet, "m") || strings.HasPrefix(wallet, "n") || strings.HasPrefix(wallet, "2") || strings.HasPrefix(wallet, "tb1")):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		W.Type = "BTC"
		W.Subtype = "TEST-NET"

	// EVM
	case stringLen(wallet) == 42 && strings.HasPrefix(wallet,"0x"):
		if(!isHex(wallet[2:])){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSumEvm(wallet)){
			W.Type = "ETH"
			W.Subtype = ""
		}else{
			return errors.New("Wallet is invalid: " + wallet)
		}


	// DOGE
	case stringLen(wallet) == 34 && strings.HasPrefix(wallet,"D"):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet, alphabetBase58)){
			W.Type = "DOGE"
			W.Subtype = "P2PKH"
		}else{
			return errors.New("Invalid checksum: "+ wallet)
		}

	case stringLen(wallet) == 34 && (strings.HasPrefix(wallet,"A") || strings.HasPrefix(wallet,"9")):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet, alphabetBase58)){
			W.Type = "DOGE"
			W.Subtype = "P2SH"
		}else{
			return errors.New("Invalid checksum: "+ wallet)
		}
	// Tron
	case stringLen(wallet) == 34 && strings.HasPrefix(wallet, "T"):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet, alphabetBase58)){
			W.Type = "TRON"
			W.Subtype = "TRC20"
		}else{
			return errors.New("Invalid checksum: "+ wallet)
		}

	// XRP uses its own base58 alphabet
	case (stringLen(wallet) >=25 && stringLen(wallet) <=35) && strings.HasPrefix(wallet, "r"):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet,alphabetBase58XRP)){
			W.Type = "XRP"
			W.Subtype = ""
		}else{
			return errors.New("Invalid checksum: "+ wallet)
		}

	// Solana: raw Ed25519 key, no checksum to verify
	case stringLen(wallet) >= 32 && stringLen(wallet) <=44:
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}

		W.Type = "SOLANA"
		W.Subtype = ""


	default:
		return errors.New("Wallet is invalid: " + wallet)
	}

	return nil
}
