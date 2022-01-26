package skill

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestDHXJSkill_Apply(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 100; i++ {
		fmt.Println(rand.Intn(len(Skills)))
	}
}
