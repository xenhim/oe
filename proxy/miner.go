package proxy

import (
	"log"
	"math/big"
	"strconv"
	"strings"

	"github.com/mcarloai/ethash"
	"github.com/ethereum/go-ethereum/common"
)

var hasher = ethash.New()

func (s *ProxyServer) processShare(login, id, ip string, t *BlockTemplate, params []string) (bool, bool) {
	nonceHex := params[0]
	hashNoNonce := params[1]
	nonce, _ := strconv.ParseUint(strings.Replace(nonceHex, "0x", "", -1), 16, 64)
	shareDiff := s.config.Proxy.Difficulty
	h, ok := t.headers[hashNoNonce]
	if !ok {
		log.Printf("Stale share from %v@%v", login, ip)
        s.backend.WriteStaleShare(login, nonceHex)
		return false, false
	}

	share := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  big.NewInt(shareDiff),
		nonce:       nonce,
		mixDigest:   common.HexToHash(""),
	}

	block := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  h.diff,
		nonce:       nonce,
		mixDigest:   common.HexToHash(""),
	}
    verified, mixDigest := hasher.Verify(share)
	if !verified {
        s.backend.WriteInvalidShare(login, nonceHex)
		return false, false
	}
    params[2] = mixDigest

    verified, _ = hasher.Verify(block)
	if verified {
		ok, err := s.rpc().SubmitBlock(params)
		if err != nil {
			log.Printf("Block submission failure at height %v for %v: %v", h.height, t.Header, err)
		} else if !ok {
			log.Printf("Block rejected at height %v for %v", h.height, t.Header)
			return false, false
		} else {
			s.fetchBlockTemplate()
			exist, err := s.backend.WriteBlock(login, id, params, shareDiff, h.diff.Int64(), h.height, s.hashrateExpiration)
			if exist {
				return true, false
			}
			if err != nil {
				log.Println("Failed to insert block candidate into backend:", err)
			} else {
				log.Printf("Inserted block %v to backend", h.height)
			}
			log.Printf("Block found by miner %v@%v at height %d", login, ip, h.height)
		}
	} else {
		exist, err := s.backend.WriteShare(login, id, params, shareDiff, h.height, s.hashrateExpiration)
		if exist {
			return true, false
		}
		if err != nil {
			log.Println("Failed to insert share data into backend:", err)
		}
	}
	return false, true
}
