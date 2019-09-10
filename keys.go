package sgx_server

import (
	"crypto/aes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	fmt "fmt"
	"io/ioutil"
	"log"
	"math/big"

	"github.com/aead/cmac"
)

func exchange(mine *ecdsa.PrivateKey, peer *ecdsa.PublicKey) []byte {
	curve := elliptic.P256()
	// only x is used as the result of key exchange
	x, _ := curve.ScalarMult(peer.X, peer.Y, mine.D.Bytes())
	xb := x.Bytes()
	if len(xb) < 32 {
		fmt.Println("Edge case")
	}
	var ret [32]byte
	copy(ret[32-len(xb):], xb)
	reverse(ret[:])
	return ret[:]
}

// TODO: implement password
func loadPrivateKey(fileName string, password string) *ecdsa.PrivateKey {
	pem_encoded, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatal("Could not open the private key file:", err)
	}

	block, _ := pem.Decode(pem_encoded)

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		log.Fatal("Could not parse the private key:", err)
	}

	return key.(*ecdsa.PrivateKey)
}

func loadPublicKey(fileName string) *ecdsa.PublicKey {
	pem_encoded, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatal("Could not open the public key file:", err)
	}

	block, _ := pem.Decode(pem_encoded)

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		log.Fatal("Could not parse the public key:", err)
	}
	return pub.(*ecdsa.PublicKey)
}

func loadKeyPair(privFile string, pubFile string, password string) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	return loadPrivateKey(privFile, password), loadPublicKey(pubFile)
}

func reverse(b []byte) {
	for left, right := 0, len(b)-1; left < right; left, right = left+1, right-1 {
		b[left], b[right] = b[right], b[left]
	}
}

func marshalPublicKey(pub *ecdsa.PublicKey) ([]byte, []byte, error) {
	var x32, y32 [32]byte
	xb, yb := pub.X.Bytes(), pub.Y.Bytes()
	copy(x32[32-len(xb):], xb)
	copy(y32[32-len(yb):], yb)
	reverse(x32[:])
	reverse(y32[:])
	return x32[:], y32[:], nil
}

func unmarshalPublicKey(xb, yb []byte) (*ecdsa.PublicKey, error) {
	// to big endian
	reverse(xb)
	reverse(yb)
	x, y := new(big.Int), new(big.Int)
	x.SetBytes(xb)
	y.SetBytes(yb)
	pub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}
	// since it's in place reverse, change it back
	reverse(xb)
	reverse(yb)
	return pub, nil
}

func generateKey() *ecdsa.PrivateKey {
	curve := elliptic.P256() // this should be SECP256R1
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Fatal("Couldn't generate an elliptic curve key.")
	}
	return priv
}

// key deriviation key
func kdk(mine *ecdsa.PrivateKey, peer *ecdsa.PublicKey) []byte {
	var cmac_key [16]byte // this always initializes to 0s in go
	shared := exchange(mine, peer)
	block, err := aes.NewCipher(cmac_key[:])
	if err != nil {
		log.Fatal("Could not create AES for CMAC", err)
	}

	key, err := cmac.Sum(shared, block, aes.BlockSize)
	if err != nil {
		log.Fatal("Could not derive the KDK", err)
	}

	return key
}

func keyDerivationString(label []byte) []byte {
	out := make([]byte, 4+len(label))
	copy(out[1:], label)
	out[0] = 1
	out[len(out)-2] = 128
	return out
}

func deriveLabelKey(mine *ecdsa.PrivateKey, peer *ecdsa.PublicKey, label []byte) ([]byte, []byte) {
	base := kdk(mine, peer)

	block, err := aes.NewCipher(base[:])
	if err != nil {
		log.Fatal("Could not create AES for CMAC", err)
	}

	key, err := cmac.Sum(keyDerivationString(label), block, aes.BlockSize)
	if err != nil {
		log.Fatal("Could not derive the KDK", err)
	}
	return base, key
}

func deriveLabelKeyFromBase(base []byte, label []byte) []byte {
	block, err := aes.NewCipher(base[:])
	if err != nil {
		log.Fatal("Could not create AES for CMAC", err)
	}

	key, err := cmac.Sum(keyDerivationString(label), block, aes.BlockSize)
	if err != nil {
		log.Fatal("Could not derive the KDK", err)
	}
	return key
}
