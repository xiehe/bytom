package tensority

import (
	"github.com/bytom/protocol/bc"
	"time"
	"fmt"
)

// Leave here for implement cache
type AIHash struct{}

func Hash(hash, seed *bc.Hash) *bc.Hash {
	fmt.Println("Initing timer for calcSeedCache")
	sT := time.Now()
	cache := calcSeedCache(seed.Bytes())
	eT := time.Now()
	fmt.Println("\tTotal time for calcSeedCache:", eT.Sub(sT))

	fmt.Println("Initing timer for mulMatrix")
	sT = time.Now()
	data := mulMatrix(hash.Bytes(), cache)
	eT = time.Now()
	fmt.Println("\tTotal time for mulMatrix:", eT.Sub(sT))

	fmt.Println("Initing timer for hashMatrix")
	sT = time.Now()
	v := hashMatrix(data)
	eT = time.Now()
	fmt.Println("\tTotal time for hashMatrix:", eT.Sub(sT))

	return v
}
