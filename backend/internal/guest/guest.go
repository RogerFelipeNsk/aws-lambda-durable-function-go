package guest

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/id"
)

var firstNames = []string{
	"Ana", "Bruno", "Camila", "Diego", "Elisa", "Felipe", "Giovana", "Henrique",
	"Isabela", "João", "Karina", "Lucas", "Marina", "Nicolas", "Olivia", "Paulo",
	"Queila", "Rafael", "Sofia", "Tiago", "Úrsula", "Vitor", "Wendy", "Yasmin",
}

var lastNames = []string{
	"Almeida", "Barbosa", "Cardoso", "Dias", "Esteves", "Ferreira", "Gomes",
	"Henriques", "Ibrahim", "Jesus", "Lima", "Mendes", "Nogueira", "Oliveira",
	"Pereira", "Rocha", "Santos", "Teixeira", "Uchoa", "Vieira",
}

func NewUser() domain.User {
	first := firstNames[mustN(len(firstNames))]
	last := lastNames[mustN(len(lastNames))]
	uid := id.New("usr")
	now := time.Now().UTC()
	return domain.User{
		ID:        uid,
		Name:      first + " " + last,
		Email:     fmt.Sprintf("visitante.%s@arena-demo.local", uid),
		CreatedAt: now,
	}
}

func mustN(n int) int {
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return int(b[0]) % n
}
