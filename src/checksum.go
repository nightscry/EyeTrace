package src

import (
	"strings"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"bytes"

	"golang.org/x/crypto/sha3"
)

// BECH32 / BECH32M
func checkSumBech(wallet string)(bool){

	hrpExampder := func (hrp string) ([]int){
		var ret []int
		for _,c := range hrp{
			ret = append(ret, int(c)>>5) // high bits
		}
		ret = append(ret, 0)
		for _, c := range hrp{
			ret = append(ret, int(c)&31) // low bits
		}
		return ret
	}

	bech32Polymod := func (values []int) (int){

		gen := []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
		chk := 1

		for _,v := range values{
			b := chk >> 25
			chk = (chk&0x1ffffff)<<5 ^ v
			for i := 0; i<5; i++{
				if(b>>i)&1 == 1{
					chk ^= gen[i]
				}
			}
		}
		return chk
	}

	var HRP string = wallet[:2]
	var dati string = wallet[3:]


	// characters to 5-bit values
	var values []int
	for _, c := range strings.ToLower(dati){
		v := strings.IndexByte(alphabetBech32,byte(c))
		if(v<0){
			return false
		}
		values = append(values,v)
	}

	all := append(hrpExampder(HRP), values...)

	res := bech32Polymod(all)



	return (res == 1 || res == 0x2bc830a3) // bech32 or bech32m
}

// EIP-55: uppercase a hex letter when its Keccak-256 nibble is >= 8
func checkSumEvm(wallet string)(bool){
	alphabet := "asdfghjklqwertyuiopzxcvbnm"
	var result string

	hash := sha3.NewLegacyKeccak256()
	lowerWallet := strings.ToLower(wallet[2:])

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


	if(result == wallet[2:]){
		return true
	}

	return false
}


// Base58Check (BTC / DOGE / TRON / XRP): last 4 bytes are sha256(sha256(payload))
func checkSum(wallet string, alphabet string)(bool){
	var num big.Int
	base := big.NewInt(58)

	for _, c := range wallet{
		index := strings.IndexRune(alphabet, c)

		num.Mul(&num, base)
		num.Add(&num, big.NewInt(int64(index)))
	}

	// leading zeros are lost by big.Int, so count and restore them
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
	if len(rawBytes) < 4 { return false}

	checksum := rawBytes[len(rawBytes)-4:]
	data := rawBytes[:len(rawBytes)-4]

	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])

	return bytes.Equal(checksum, hash2[:4])
}
