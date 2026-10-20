package action

type Action interface{}

type DrawCardsAction struct {
	amount int
}

func NewDrawCardsAction(amount int) Action {
	return DrawCardsAction{amount: amount}
}

func (a DrawCardsAction) Amount() int {
	return a.amount
}

type ReverseTurnsAction struct{}

func NewReverseTurnsAction() Action {
	return ReverseTurnsAction{}
}

type SkipTurnAction struct{}

func NewSkipTurnAction() Action {
	return SkipTurnAction{}
}

type PickColorAction struct{}

func NewPickColorAction() Action {
	return PickColorAction{}
}
