package shortener

// main logic component (Control)
// lots of thought for the decision of url mapping
// I will explain it in Decisions.md once I had the "Hal" to do it :)
// but in short explaination: We will have 3 different hash functions:
// hashTo6, hashTo7, hashTo8 which any of these "hashToN"s will hash an unlimited-length url to N chars of Base62 (for Code)
// given the original url (called origin), first we'll map it with hashTo6 and if there were no collision we're fine.
// in case of any collision, then we will map it with hashTo7 (a completley different code) and if there were no collision
// in universe of 7-chars codes, then we are fine. Hopefully we will not need more as if two long url (A and B and C)
// collide on hashTo6 mapping, the chance of a collision between B and C in hashTo7 mapping is extremly low.
// but still if that happend as well, then we will move on to hashTo8 mapping strategy :))
// for now I don't want to overengineer it for more corner case (collision on hashTo8 mapping as well)
// but the idea for that can be adding some meaningless / or ? at the end of the url to change the mapping result
// or even adding a counter (like <code>0, <code>1, ... <code>Z, <code>00, ...)
// but for now we will through an error or something like that in the case of happening of that :)
// sorry for my bad english but I added these self-written comments to prove it is not AI generated comment or code ^_^

import (
	"errors"
	"hash/fnv"
	"math/big"
	"strings"
	"url-shortener/internal/store"
)

const key string = "hi hello how are you!"

type Shortener struct {
	db store.Store
}

// Returns an instance of shortener logic center
// needs massive refactore
func New() *Shortener {
	// db: RAM (in-memory), later gets it as a parameter
	return &Shortener{db: store.NewRam()}
}

// cleaner implementation of the shortening algorithm (code only)
func (s *Shortener) Shorten(originalURL string) (string, error) {
	// generate a short ID, explained in DESCISIONS.md
	for length := 6; length <= 8; length++ {
		id := hashToN(originalURL, key, length)
		urlInDB, found := s.db.Read(id)
		if !found {
			err := s.db.Write(id, originalURL)
			return id, err
		}
		if urlInDB == originalURL {
			return id, nil // Idempotent match
		}
		// Collision: continue to next length
	}
	return "", errors.New("collision detected across all code lengths (6-8)")
}

func (s *Shortener) Redirect(id string) (string, bool) {
	return s.db.Read(id) // originalURL, found/exists
}

// hashes an input to the N-digit code in base62 (fixed size hash using fnv 64 bit & bitmasking)
func hashToN(input string, key string, length int) string {
	hasher := fnv.New64()
	hasher.Write([]byte(key))
	hasher.Write([]byte(input))
	hashUint64 := hasher.Sum64()

	maxBace62Value := new(big.Int).Exp(big.NewInt(62), big.NewInt(int64(length)), nil)

	biHash := new(big.Int).SetUint64(hashUint64)
	finalNumber := new(big.Int).Mod(biHash, maxBace62Value)

	base62str := finalNumber.Text(62)

	if len(base62str) < length {
		base62str = strings.Repeat("0", length-len(base62str)) + base62str
	}
	return base62str
}
