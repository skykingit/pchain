package crypto

import (
	"bytes"

	secp256k1 "github.com/btcsuite/btcd/btcec"
	"github.com/tendermint/ed25519"
	"github.com/tendermint/ed25519/extra25519"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-data"
	"github.com/tendermint/go-wire"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"bls"
)

// PrivKey is part of PrivAccount and state.PrivValidator.
type PrivKey interface {
	Bytes() []byte
	Sign(msg []byte) Signature
	PubKey() PubKey
	Equals(PrivKey) bool
}

// Types of implementations
const (
	TypeEd25519   = byte(0x01)
	TypeSecp256k1 = byte(0x02)
	TypeEthereum   = byte(0x03)
	TypeBls       = byte(0x04)
	NameEd25519   = "ed25519"
	NameSecp256k1 = "secp256k1"
	NameEthereum    = "ethereum"
	NameBls        = "bls"
)

var privKeyMapper data.Mapper

// register both private key types with go-data (and thus go-wire)
func init() {
	privKeyMapper = data.NewMapper(PrivKeyS{}).
		RegisterImplementation(PrivKeyEd25519{}, NameEd25519, TypeEd25519).
		RegisterImplementation(PrivKeySecp256k1{}, NameSecp256k1, TypeSecp256k1).
		RegisterImplementation(EthereumPrivKey{}, NameEthereum, TypeEthereum).
		RegisterImplementation(BLSPrivKey{}, NameBls, TypeBls)

}

// PrivKeyS add json serialization to PrivKey
type PrivKeyS struct {
	PrivKey
}

func WrapPrivKey(pk PrivKey) PrivKeyS {
	for ppk, ok := pk.(PrivKeyS); ok; ppk, ok = pk.(PrivKeyS) {
		pk = ppk.PrivKey
	}
	return PrivKeyS{pk}
}

func (p PrivKeyS) MarshalJSON() ([]byte, error) {
	return privKeyMapper.ToJSON(p.PrivKey)
}

func (p *PrivKeyS) UnmarshalJSON(data []byte) (err error) {
	parsed, err := privKeyMapper.FromJSON(data)
	if err == nil && parsed != nil {
		p.PrivKey = parsed.(PrivKey)
	}
	return
}

func (p PrivKeyS) Empty() bool {
	return p.PrivKey == nil
}

func PrivKeyFromBytes(privKeyBytes []byte) (privKey PrivKey, err error) {
	err = wire.ReadBinaryBytes(privKeyBytes, &privKey)
	return
}

//-------------------------------------

// Implements PrivKey
type PrivKeyEd25519 [64]byte

func (privKey PrivKeyEd25519) Bytes() []byte {
	return wire.BinaryBytes(struct{ PrivKey }{privKey})
}

func (privKey PrivKeyEd25519) Sign(msg []byte) Signature {
	privKeyBytes := [64]byte(privKey)
	signatureBytes := ed25519.Sign(&privKeyBytes, msg)
	return SignatureEd25519(*signatureBytes)
}

func (privKey PrivKeyEd25519) PubKey() PubKey {
	privKeyBytes := [64]byte(privKey)
	return PubKeyEd25519(*ed25519.MakePublicKey(&privKeyBytes))
}

func (privKey PrivKeyEd25519) Equals(other PrivKey) bool {
	if otherEd, ok := other.(PrivKeyEd25519); ok {
		return bytes.Equal(privKey[:], otherEd[:])
	} else {
		return false
	}
}

func (p PrivKeyEd25519) MarshalJSON() ([]byte, error) {
	return data.Encoder.Marshal(p[:])
}

func (p *PrivKeyEd25519) UnmarshalJSON(enc []byte) error {
	var ref []byte
	err := data.Encoder.Unmarshal(&ref, enc)
	copy(p[:], ref)
	return err
}

func (privKey PrivKeyEd25519) ToCurve25519() *[32]byte {
	keyCurve25519 := new([32]byte)
	privKeyBytes := [64]byte(privKey)
	extra25519.PrivateKeyToCurve25519(keyCurve25519, &privKeyBytes)
	return keyCurve25519
}

func (privKey PrivKeyEd25519) String() string {
	return Fmt("PrivKeyEd25519{*****}")
}

// Deterministically generates new priv-key bytes from key.
func (privKey PrivKeyEd25519) Generate(index int) PrivKeyEd25519 {
	newBytes := wire.BinarySha256(struct {
		PrivKey [64]byte
		Index   int
	}{privKey, index})
	var newKey [64]byte
	copy(newKey[:], newBytes)
	return PrivKeyEd25519(newKey)
}

func GenPrivKeyEd25519() PrivKeyEd25519 {
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], CRandBytes(32))
	ed25519.MakePublicKey(privKeyBytes)
	return PrivKeyEd25519(*privKeyBytes)
}

// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeyEd25519FromSecret(secret []byte) PrivKeyEd25519 {
	privKey32 := Sha256(secret) // Not Ripemd160 because we want 32 bytes.
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], privKey32)
	ed25519.MakePublicKey(privKeyBytes)
	return PrivKeyEd25519(*privKeyBytes)
}

//-------------------------------------

// Implements PrivKey
type PrivKeySecp256k1 [32]byte

func (privKey PrivKeySecp256k1) Bytes() []byte {
	return wire.BinaryBytes(struct{ PrivKey }{privKey})
}

func (privKey PrivKeySecp256k1) Sign(msg []byte) Signature {
	priv__, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	sig__, err := priv__.Sign(Sha256(msg))
	if err != nil {
		PanicSanity(err)
	}
	return SignatureSecp256k1(sig__.Serialize())
}

func (privKey PrivKeySecp256k1) PubKey() PubKey {
	_, pub__ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	var pub PubKeySecp256k1
	copy(pub[:], pub__.SerializeCompressed())
	return pub
}

func (privKey PrivKeySecp256k1) Equals(other PrivKey) bool {
	if otherSecp, ok := other.(PrivKeySecp256k1); ok {
		return bytes.Equal(privKey[:], otherSecp[:])
	} else {
		return false
	}
}

func (p PrivKeySecp256k1) MarshalJSON() ([]byte, error) {
	return data.Encoder.Marshal(p[:])
}

func (p *PrivKeySecp256k1) UnmarshalJSON(enc []byte) error {
	var ref []byte
	err := data.Encoder.Unmarshal(&ref, enc)
	copy(p[:], ref)
	return err
}

func (privKey PrivKeySecp256k1) String() string {
	return Fmt("PrivKeySecp256k1{*****}")
}


type EthereumPrivKey []byte

func (privKey EthereumPrivKey) Bytes() []byte {
	return wire.BinaryBytes(struct{ PrivKey }{privKey})
}

func (privKey EthereumPrivKey) Sign(msg []byte) Signature {
	priv, err := ethcrypto.ToECDSA(privKey)
	if err != nil {
		return nil
	}
	msg = ethcrypto.Keccak256(msg)
	sig, err := ethcrypto.Sign(msg, priv)
	if err != nil {
		return nil
	}
	return EthereumSignature(sig)
}

func (privKey EthereumPrivKey) PubKey() PubKey {
	priv, err := ethcrypto.ToECDSA(privKey)
	if err != nil {
		panic(err)
	}
	pubKey := ethcrypto.FromECDSAPub(&priv.PublicKey)
	return EthereumPubKey(pubKey)
}

func (privKey EthereumPrivKey) Equals(other PrivKey) bool {
	if otherEd, ok := other.(EthereumPrivKey); ok {
		return bytes.Equal(privKey[:], otherEd[:])
	} else {
		return false
	}
}


func (privKey EthereumPrivKey) MarshalJSON() ([]byte, error) {
	return data.Encoder.Marshal(privKey[:])
}


func (privKey *EthereumPrivKey) UnmarshalJSON(enc []byte) error {
	var ref []byte
	err := data.Encoder.Unmarshal(&ref, enc)
	copy((*privKey)[:], ref)
	return err
}

/*
// Deterministically generates new priv-key bytes from key.
func (key PrivKeySecp256k1) Generate(index int) PrivKeySecp256k1 {
	newBytes := wire.BinarySha256(struct {
		PrivKey [64]byte
		Index   int
	}{key, index})
	var newKey [64]byte
	copy(newKey[:], newBytes)
	return PrivKeySecp256k1(newKey)
}
*/

func GenPrivKeySecp256k1() PrivKeySecp256k1 {
	privKeyBytes := [32]byte{}
	copy(privKeyBytes[:], CRandBytes(32))
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKeyBytes[:])
	copy(privKeyBytes[:], priv.Serialize())
	return PrivKeySecp256k1(privKeyBytes)
}

// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeySecp256k1FromSecret(secret []byte) PrivKeySecp256k1 {
	privKey32 := Sha256(secret) // Not Ripemd160 because we want 32 bytes.
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey32)
	privKeyBytes := [32]byte{}
	copy(privKeyBytes[:], priv.Serialize())
	return PrivKeySecp256k1(privKeyBytes)
}


//-------------------------------------
// Implements PrivKey
/*
func init() {
	paramsString := "type a\n"+
		"q 6810019449936382487924444340676335792486684152989565749316517380074446105713919870191983352817997578314362713972330796409852501768474201988787261287855671\n"+
		"h 9319208786345675094887650862530830862537242472377645595444311486184402577395907493359426358133397009566152\n"+
		"r 730750818665452757176057050065048642452048576511\n"+
		"exp2 159\n"+
		"exp1 110\n"+
		"sign1 1\n"+
		"sign0 -1\n"
	gString := "0x22a80aa68ea4a9777a7417bd3e9508a6f9f4fe1d6b2c729e04cbdcf85f44e84be91edc6ee1fd10567cd784d0afc2b23ae14ec363ca300113b8b0553d399997942bdb71a1366b807ffa573eccb7d26ef61bf022d7d24a19a14ac605e092cea37cb5b59804e2d7aa9db9bf572904142c0652883c6d300a1c02dc6c4fdf4ca44585"
	pairing_, pair_err := pbc.NewPairingFromString(paramsString)
	if pair_err != nil {

	}
	g_ := pairing_.NewG2().SetBytes(common.FromHex(gString))
	pairing = pairing_
	g = g_
}

var pairing *pbc.Pairing
var g *pbc.Element

func Pairing() *pbc.Pairing {
	return pairing
}

type BLSPrivKey []byte

func CreateBLSPrivKey() BLSPrivKey {
	privKey := pairing.NewZr().Rand()
	return privKey.Bytes()
}

func (privKey BLSPrivKey) Bytes() []byte {
	return privKey
}


func (privKey BLSPrivKey) getElement() *pbc.Element {
	return pairing.NewZr().SetBytes(privKey)
}

func (privKey BLSPrivKey) GetElement() *pbc.Element {
	return pairing.NewZr().SetBytes(privKey)
}

func (privKey BLSPrivKey) PubKey() PubKey {
	pubKey := pairing.NewG2().PowZn(g, privKey.getElement())
	return BLSPubKey(pubKey.Bytes())
}

func (privKey BLSPrivKey) Sign(msg []byte) Signature {
	h := pairing.NewG1().SetFromStringHash(string(msg), sha256.New())
	signature := pairing.NewG2().PowZn(h, privKey.getElement())
	return BLSSignature(signature.Bytes())
}

func (privKey BLSPrivKey) Equals(other PrivKey) bool {
	if otherKey,ok := other.(BLSPrivKey); ok {
		return privKey.getElement().Equals(otherKey.getElement())
	} else {
		return false
	}
}

func (privKey BLSPrivKey) MarshalJSON() ([]byte, error) {
	return data.Encoder.Marshal(privKey)
}

func (privKey *BLSPrivKey) UnmarshalJSON(enc []byte) error {
	var ref []byte
	err := data.Encoder.Unmarshal(&ref, enc)
	copy(*privKey, ref)
	return err
}*/

type BLSPrivKey []byte
func (privKey BLSPrivKey) Bytes() []byte {
	return privKey
}

func (privKey BLSPrivKey) getElement() *bls.PrivateKey {
	sk := &bls.PrivateKey{}
	err := sk.Unmarshal(privKey)
	if err != nil {
		return nil
	} else {
		return sk
	}
}

func (privKey BLSPrivKey) PubKey() PubKey {
	pubKey := privKey.getElement().Public()
	var pub BLSPubKey
	copy(pub[:], pubKey.Marshal())
	return pub
}

func (privKey BLSPrivKey) Sign(msg []byte) Signature {
	sk := privKey.getElement()
	sign := bls.Sign(sk, msg)
	return BLSSignature(sign.Marshal())
}

func (privKey BLSPrivKey) Equals(other PrivKey) bool {
	if otherSk,ok := other.(BLSPrivKey); ok {
		return bytes.Equal(privKey, otherSk)
	} else {
		return false
	}
}

func (privKey BLSPrivKey) MarshalJSON() ([]byte, error) {
	return data.Encoder.Marshal(privKey)
}

func (privKey *BLSPrivKey) UnmarshalJSON(enc []byte) error {
	var ref []byte
	err := data.Encoder.Unmarshal(&ref, enc)
	copy(*privKey, ref)
	return err
}