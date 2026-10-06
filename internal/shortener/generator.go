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
	"fmt"
	"url-shortener/internal/store"
)

type Shortener struct {
	db store.Store
}

// Returns an instance of shortener logic center
// needs massive refactore
func New() *Shortener {
	// db: RAM (in-memory), later gets it as a parameter
	return &Shortener{db: store.NewRam()}
}

func (s *Shortener) Shorten(originalURL string) string {
	// generate a short ID (simple counter for Phase1)
	id := fmt.Sprintf("%d", 1)      // later we can use a better ID generation method (and safer!)
	_ = s.db.Write(id, originalURL) // error ignored for now
	return id
}

func (s *Shortener) Redirect(id string) (string, bool) {
	return s.db.Read(id) // originalURL, found/exists
}
