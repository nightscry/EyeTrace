package src

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/sha3"
)

const alphabetBase58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
const alphabetBase58XRP = "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz"
const alphabetHex = "0123456789abcdefABCDEF"

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
	
	for _, c := range wallet{
		if(!strings.Contains(alphabetBase58,string(c))){
			return false
		}
	}

	return true
	
}

//checksum EVM
func checkSumEvm(wallet string)(bool){
	alphabet := "asdfghjklqwertyuiopzxcvbnm"

	var result string

	hash := sha3.NewLegacyKeccak256()

	lowerWallet := strings.ToLower(wallet[2:])
	fmt.Println(lowerWallet)

	hash.Write([]byte(lowerWallet))
	keccakHashRaw := hash.Sum(nil)
	keccakHash := hex.EncodeToString(keccakHashRaw)
	
	
	for i, c := range lowerWallet{

		if(strings.Contains(alphabet,string(c))){
			charHash := keccakHash[i]
			if(strings.Contains("89abcdef",string(charHash))){
				result += strings.ToUpper(string(c))
			}else{
				result += string(c)
			}
		}else{
			result += string(c)
		}

	}
	fmt.Println("Calcolato: "+result)
	fmt.Println("Orginale: "+ wallet)

	if(result == wallet[2:]){
		return true
	}

	return false
}


//checksum checker BTC/DOGE/TRON
func checkSum(wallet string, alphabet string)(bool){
	var num big.Int
	base := big.NewInt(58)

	for _, c := range wallet{
		index := strings.IndexRune(alphabet, c)

		num.Mul(&num, base)
		num.Add(&num, big.NewInt(int64(index)))
	}

	zeros := 0
	t := alphabet[0]
	for _, c := range wallet {
		if c != rune(t) {
			break 
		}
		zeros++
	}

	rawBytes := make([]byte, zeros)
	rawBytes = append(rawBytes, num.Bytes()...)

	//checksum
	checksum := rawBytes[len(rawBytes)-4:]
	
	//version + paylaod
	data := rawBytes[:len(rawBytes)-4]

	//calc the checksum
	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])

	return bytes.Equal(checksum, hash2[:4])
}


//HEX checker
func isHex(payload string)(bool){
	
	for _, c := range payload{
		if(!strings.Contains(alphabetHex,string(c))){
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
		if(checkSum(wallet,alphabetBase58)){
			W.Type = "BTC"
			W.Subtype = "P2PKH"
		}else{
			return errors.New("CheckSum invalid of this: "+ wallet)
		}
		
		

	case strings.HasPrefix(wallet,"3") && stringLen(wallet) == 34:
		//REMOVE TRASH
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet,alphabetBase58)){
			W.Type = "BTC"
			W.Subtype = "P2SH"
		}else{
			return errors.New("CheckSum invalid of this: "+ wallet)
		}
		
		
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
			return errors.New("Wallet is invalid: " + wallet)
		}
		W.Type = "BTC"
		W.Subtype = "TEST-NET"

	//check if is EVM
	case stringLen(wallet) == 42 && strings.HasPrefix(wallet,"0x"):
		if(!isHex(wallet[2:])){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSumEvm(wallet)){
			W.Type = "EVM"
			W.Subtype = ""
		}else{
			return errors.New("Wallet is invalid: " + wallet)
		}


	//check if is dogecoin
	case stringLen(wallet) == 34 && strings.HasPrefix(wallet,"D"):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet, alphabetBase58)){
			W.Type = "DOGE"
			W.Subtype = "P2PKH"
		}else{
			return errors.New("CheckSum invalid of this: "+ wallet)
		}
	
	case stringLen(wallet) == 34 && (strings.HasPrefix(wallet,"A") || strings.HasPrefix(wallet,"9")):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet, alphabetBase58)){
			W.Type = "DOGE"
			W.Subtype = "P2SH"
		}else{
			return errors.New("CheckSum invalid of this: "+ wallet)
		}
	//check if is Tron
	case stringLen(wallet) == 34 && strings.HasPrefix(wallet, "T"):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet, alphabetBase58)){
			W.Type = "TRON"
			W.Subtype = "TRC20"
		}else{
			return errors.New("CheckSum invalid of this: "+ wallet)
		}
	
	//check if is XRP
	case (stringLen(wallet) >=25 && stringLen(wallet) <=35) && strings.HasPrefix(wallet, "r"):
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		if(checkSum(wallet,alphabetBase58XRP)){
			W.Type = "XRP"
			W.Subtype = ""
		}else{
			return errors.New("CheckSum invalid of this: "+ wallet)
		}
	
	//check if is solana
	case stringLen(wallet) >= 32 && stringLen(wallet) <=44:
		if(!isBase58(wallet)){
			return errors.New("Wallet is invalid: " + wallet)
		}
		W.Type = "SOLANA"
		W.Subtype = ""

	default:
		W.Type = "NONE"
		W.Subtype = "NONE"
	}

	return  nil
}
