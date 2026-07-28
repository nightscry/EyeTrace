package src

import "strings"


const alphabetBase58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
const alphabetBase58XRP = "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz"
const alphabetBech32 = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
const alphabetHex = "0123456789abcdefABCDEF"



func stringLen(str string)(int){
	var l int
	for range str{
		l++
	}
	return l
}

// Base58 excludes the look-alike characters 0, O, I and l.
func isBase58(wallet string)(bool){

	for _, c := range wallet{
		if(!strings.Contains(alphabetBase58,string(c))){
			return false
		}
	}

	return true

}


func isHex(payload string)(bool){

	for _, c := range payload{
		if(!strings.Contains(alphabetHex,string(c))){
			return false
		}
	}
	return true
}
